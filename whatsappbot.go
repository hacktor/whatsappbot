package main

import (
    "encoding/gob"
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"
    "strconv"
    "strings"
    "database/sql"
    _ "github.com/mattn/go-sqlite3"

    qrcodeTerminal "github.com/Baozisoftware/qrcode-terminal-go"
    "github.com/Rhymen/go-whatsapp/binary/proto"
    "github.com/Rhymen/go-whatsapp"
    "github.com/pelletier/go-toml"
    "github.com/hpcloud/tail"
    "net/http"
    "net/url"
)

type waHandler struct {
    c *whatsapp.Conn
}

type config struct {
    groupid  string
    infile   string
    db       string
    attach   string
    url      string
    session  string
    ircfile  string
    sigfile  string
    matfile  string
    teltoken string
    telchat  string
    anon     string
}

var cfg config
var StartTime = uint64(time.Now().Unix())

//HandleError needs to be implemented to be a valid WhatsApp handler
func (h *waHandler) HandleError(err error) {

    if e, ok := err.(*whatsapp.ErrConnectionFailed); ok {
        log.Printf("Connection failed, underlying error: %v", e.Err)
        log.Println("Waiting 30sec...")
        <-time.After(30 * time.Second)
        log.Println("Reconnecting...")
        err := h.c.Restore()
        if err != nil {
            log.Fatalf("Restore failed: %v", err)
        }
    } else {
        log.Printf("error occoured: %v\n", err)
    }
}

//Optional to be implemented. Implement HandleXXXMessage for the types you need.
func (*waHandler) HandleTextMessage(m whatsapp.TextMessage) {

    if m.Info.Timestamp < StartTime {
        fmt.Printf("Skipping old message (%v) with timestamp %v\n", m.Text, m.Info.Timestamp)
        return
    }

    if m.Info.RemoteJid != cfg.groupid {
        fmt.Printf("RemoteJid %v does not match groupid %v, skipping\n", m.Info.RemoteJid, cfg.groupid)
        return
    }

    fmt.Printf("Timestamp: %v; ID: %v; Group: %v; Sender: %v; Text: %v\n",
        m.Info.Timestamp, m.Info.Id, m.Info.RemoteJid, *m.Info.Source.Participant, m.Text)

    sender := getNick(*m.Info.Source.Participant)

    //relay to irc, signal, matrix
    relay(sender, m.Text)

    //scan for !setnick command
    if len(m.Text) > 8 && m.Text[:8] == "!setnick" {

        parts := strings.Fields(m.Text)
        setNick(*m.Info.Source.Participant, strings.Join(parts[1:], " "))
    }
}

//Image handling. Video, Audio, Document are also possible in the same way
func (h *waHandler) HandleImageMessage(m whatsapp.ImageMessage) {

    if m.Info.Timestamp < StartTime {
        fmt.Printf("Skipping old message (%v) with timestamp %v\n")
        return
    }

    if m.Info.RemoteJid != cfg.groupid {
        fmt.Printf("RemoteJid %v does not match groupid %v, skipping\n", m.Info.RemoteJid, cfg.groupid)
        return
    }

    data, e := m.Download()
    if e != nil {
        if e != whatsapp.ErrMediaDownloadFailedWith410 && e != whatsapp.ErrMediaDownloadFailedWith404 {
            return
        }
        if _, e = h.c.LoadMediaInfo(m.Info.RemoteJid, m.Info.Id, strconv.FormatBool(m.Info.FromMe)); e == nil {
            data, e = m.Download()
            if e != nil {
                return
            }
        }
    }

    filename := fmt.Sprintf("%v.%v", m.Info.Id, strings.Split(m.Type, "/")[1])
    file, e := os.Create(cfg.attach + "/" + filename)
    defer file.Close()
    if e != nil {
        return
    }
    _, e = file.Write(data)
    if e != nil {
        return
    }
    log.Printf("%v %v\n\timage received, saved at: %v/%v\n", m.Info.Timestamp, m.Info.RemoteJid, cfg.attach, filename)
    sender := getNick(*m.Info.Source.Participant)
    sender = "**" + sender
    text := " sends an image: " + cfg.url + "/" + filename
    if len(m.Caption) > 0 {
        text += " with caption: " + m.Caption
    }

    //relay to irc, signal, matrix
    relay(sender, text)
}

func main() {

    //get configuration
    if t, e := toml.LoadFile("/etc/hermod.toml"); e != nil {
        log.Fatalf("error loading configuration: %v\n", e)
    } else {
        cfg = config{
            groupid:  t.Get("whatsapp.groupid").(string),
            infile:   t.Get("whatsapp.infile").(string),
            db:       t.Get("whatsapp.db").(string),
            attach:   t.Get("whatsapp.attachments").(string),
            url:      t.Get("whatsapp.url").(string),
            session:  t.Get("whatsapp.session").(string),
            ircfile:  t.Get("irc.infile").(string),
            sigfile:  t.Get("signal.infile").(string),
            matfile:  t.Get("matrix.infile").(string),
            teltoken: t.Get("telegram.token").(string),
            telchat:  t.Get("telegram.chat_id").(string),
            anon:     t.Get("common.anon").(string),
        }
    }

    //create new WhatsApp connection
    wac, e := whatsapp.NewConn(5 * time.Second)
    if e != nil {
        log.Fatalf("error creating connection: %v\n", e)
    }

    //Add handler
    wac.AddHandler(&waHandler{wac})

    //login or restore
    if e := login(wac); e != nil {
        log.Fatalf("error logging in: %v\n", e)
    }

    //verifies phone connectivity
    pong, e := wac.AdminTest()

    if !pong || e != nil {
        log.Fatalf("error pinging in: %v\n", e)
    }

    //start reading infile
    go infile(wac)

    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt, syscall.SIGTERM)
    <-c

    //Disconnect safe
    fmt.Println("Shutting down now.")
    session, e := wac.Disconnect()
    if e != nil {
        log.Fatalf("error disconnecting: %v\n", e)
    }
    if e := writeSession(session); e != nil {
        log.Fatalf("error saving session: %v", e)
    }
}

func relay(sender string, msg string) {

    //relay to irc, signal, matrix
    bridges := [3]string{cfg.ircfile, cfg.sigfile,cfg.matfile}
    msg = "[wha] " + sender + ": " + msg + "\n"
    for _, infile := range bridges {

        f, e := os.OpenFile(infile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
        if e != nil {
            continue
        }
        if _, e := f.Write([]byte(msg)); e != nil {
            log.Println(e)
            continue
        }
        if e := f.Close(); e != nil {
            log.Println(e)
            continue
        }
    }

    // relay to telegram
    telurl := "https://api.telegram.org/bot" + cfg.teltoken + "/sendMessage?chat_id=" + cfg.telchat + "&text="
    if _, e := http.Get(telurl + url.QueryEscape(msg)); e != nil {
        log.Printf("Telegram fail: %v\n", e)
    }
}

func getNick(sender string) string {

    var anon = getAnon(sender)
    var phone string
    var nick string

    //open database
    db, e := sql.Open("sqlite3", cfg.db)
    if e != nil {
        log.Fatalf("Error opening db: %v\n", e)
    }
    defer db.Close()

    rows, e := db.Query("SELECT phone, nick FROM alias")
    if e != nil {
        log.Printf("Query failed: %v\n", e)
        return anon
    }
    defer rows.Close()

    for rows.Next() {

        e = rows.Scan(&phone, &nick)
        if e != nil {
            return anon
        }
        if phone == sender {

            // found a nick
            return nick
        }
    }
    return anon
}

func getAnon(sender string) string {

    //anonymize telephone number
    sender = strings.Split(sender,"@")[0]
    if len(sender) > 8 {
        return cfg.anon + "-" + sender[7:]
    } else {
        return "Anonymous"
    }
}

func setNick(sender string, nick string) {

    //open database
    db, e := sql.Open("sqlite3", cfg.db)
    if e != nil {
        log.Fatalf("Error opening db: %v\n", e)
    }
    defer db.Close()

    stmt, e := db.Prepare("REPLACE INTO alias (phone, nick) values (?,?)")
    if e != nil {
        log.Printf("Prepare failed: %v\n", e)
        return
    }
    defer stmt.Close()

    _, e = stmt.Exec(sender, nick)
    if e != nil {
        log.Printf("Insert nick failed: %v\n", e)
    }
}

func infile(wac *whatsapp.Conn) {

    // keep a tail on the infile
    loc := &tail.SeekInfo{Offset: 0, Whence: os.SEEK_END}
    if t, e := tail.TailFile(cfg.infile, tail.Config{Follow: true, ReOpen: true, Location: loc}); e == nil {

        for line := range t.Lines {
            fmt.Println(line.Text)

            prevMessage := "?"
            quotedMessage := proto.Message{
                Conversation: &prevMessage,
            }

            ContextInfo := whatsapp.ContextInfo{
                QuotedMessage:   &quotedMessage,
                QuotedMessageID: "",
                Participant:     "", //Whot sent the original message
            }

            msg := whatsapp.TextMessage{
                Info: whatsapp.MessageInfo{
                    RemoteJid: cfg.groupid,
                },
                ContextInfo: ContextInfo,
                Text:        line.Text,
            }

            if msgId, e := wac.Send(msg); e != nil {
                log.Fatalf("error sending message: %v\n", e)
            } else {
                fmt.Printf("Message Sent -> ID : %v\n", msgId)
            }

        }

    } else {
        fmt.Printf("Error in tail file: %v\n", e)
    }
}

func login(wac *whatsapp.Conn) error {
    //load saved session
    session, e := readSession()
    if e == nil {
        //restore session
        session, e = wac.RestoreWithSession(session)
        if e != nil {
            return fmt.Errorf("restoring failed: %v\n", e)
        }
    } else {
        //no saved session -> regular login
        qr := make(chan string)
        go func() {
            terminal := qrcodeTerminal.New()
            terminal.Get(<-qr).Print()
        }()
        session, e = wac.Login(qr)
        if e != nil {
            return fmt.Errorf("error during login: %v\n", e)
        }
    }

    //save session
    e = writeSession(session)
    if e != nil {
        return fmt.Errorf("error saving session: %v\n", e)
    }
    return nil
}

func readSession() (whatsapp.Session, error) {
    session := whatsapp.Session{}
    file, e := os.Open(cfg.session)
    if e != nil {
        return session, e
    }
    defer file.Close()
    decoder := gob.NewDecoder(file)
    e = decoder.Decode(&session)
    if e != nil {
        return session, e
    }
    return session, nil
}

func writeSession(session whatsapp.Session) error {
    file, e := os.Create(cfg.session)
    if e != nil {
        return e
    }
    defer file.Close()
    encoder := gob.NewEncoder(file)
    e = encoder.Encode(session)
    if e != nil {
        return e
    }
    return nil
}

