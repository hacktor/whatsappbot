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
    attach   string
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
    sender := getSender(*m.Info.Source.Participant)

    //relay to irc, signal, matrix
    relay(sender, m.Text)
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

    data, err := m.Download()
    if err != nil {
        if err != whatsapp.ErrMediaDownloadFailedWith410 && err != whatsapp.ErrMediaDownloadFailedWith404 {
            return
        }
        if _, err = h.c.LoadMediaInfo(m.Info.RemoteJid, m.Info.Id, strconv.FormatBool(m.Info.FromMe)); err == nil {
            data, err = m.Download()
            if err != nil {
                return
            }
        }
    }

    filename := fmt.Sprintf("%v/%v.%v", cfg.attach, m.Info.Id, strings.Split(m.Type, "/")[1])
    file, err := os.Create(filename)
    defer file.Close()
    if err != nil {
        return
    }
    _, err = file.Write(data)
    if err != nil {
        return
    }
    log.Printf("%v %v\n\timage received, saved at:%v\n", m.Info.Timestamp, m.Info.RemoteJid, filename)
}

func main() {

    //get configuration
    if t, err := toml.LoadFile("/etc/hermod.toml"); err != nil {
        log.Fatalf("error loading configuration: %v\n", err)
    } else {
        cfg = config{
            groupid:  t.Get("whatsapp.groupid").(string),
            infile:   t.Get("whatsapp.infile").(string),
            attach:   t.Get("whatsapp.attachments").(string),
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
    wac, err := whatsapp.NewConn(5 * time.Second)
    if err != nil {
        log.Fatalf("error creating connection: %v\n", err)
    }

    //Add handler
    wac.AddHandler(&waHandler{wac})

    //login or restore
    if err := login(wac); err != nil {
        log.Fatalf("error logging in: %v\n", err)
    }

    //verifies phone connectivity
    pong, err := wac.AdminTest()

    if !pong || err != nil {
        log.Fatalf("error pinging in: %v\n", err)
    }

    //start reading infile
    go infile(wac)

    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt, syscall.SIGTERM)
    <-c

    //Disconnect safe
    fmt.Println("Shutting down now.")
    session, err := wac.Disconnect()
    if err != nil {
        log.Fatalf("error disconnecting: %v\n", err)
    }
    if err := writeSession(session); err != nil {
        log.Fatalf("error saving session: %v", err)
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

func getSender(s string) string {

    //anonymize telephone number
    s = strings.Split(s,"@")[0]
    return cfg.anon + "-" + s[7:]
}

func infile(wac *whatsapp.Conn) {

    // keep a tail on the infile
    loc := &tail.SeekInfo{Offset: 0, Whence: os.SEEK_END}
    if t, err := tail.TailFile(cfg.infile, tail.Config{Follow: true, ReOpen: true, Location: loc}); err == nil {

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

            if msgId, err := wac.Send(msg); err != nil {
                log.Fatalf("error sending message: %v\n", err)
            } else {
                fmt.Printf("Message Sent -> ID : %v\n", msgId)
            }

        }

    } else {
        fmt.Printf("Error in tail file: %v\n", err)
    }
}

func login(wac *whatsapp.Conn) error {
    //load saved session
    session, err := readSession()
    if err == nil {
        //restore session
        session, err = wac.RestoreWithSession(session)
        if err != nil {
            return fmt.Errorf("restoring failed: %v\n", err)
        }
    } else {
        //no saved session -> regular login
        qr := make(chan string)
        go func() {
            terminal := qrcodeTerminal.New()
            terminal.Get(<-qr).Print()
        }()
        session, err = wac.Login(qr)
        if err != nil {
            return fmt.Errorf("error during login: %v\n", err)
        }
    }

    //save session
    err = writeSession(session)
    if err != nil {
        return fmt.Errorf("error saving session: %v\n", err)
    }
    return nil
}

func readSession() (whatsapp.Session, error) {
    session := whatsapp.Session{}
    file, err := os.Open(cfg.session)
    if err != nil {
        return session, err
    }
    defer file.Close()
    decoder := gob.NewDecoder(file)
    err = decoder.Decode(&session)
    if err != nil {
        return session, err
    }
    return session, nil
}

func writeSession(session whatsapp.Session) error {
    file, err := os.Create(cfg.session)
    if err != nil {
        return err
    }
    defer file.Close()
    encoder := gob.NewEncoder(file)
    err = encoder.Encode(session)
    if err != nil {
        return err
    }
    return nil
}

