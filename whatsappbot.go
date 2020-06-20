package main

import (
    "encoding/gob"
    "flag"
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"
    "strconv"
    "strings"

    qrcodeTerminal "github.com/Baozisoftware/qrcode-terminal-go"
    "github.com/Rhymen/go-whatsapp"
    "net/http"
    "net/url"
)

type waHandler struct {
    c *whatsapp.Conn
    wait chan int64
}

var cfg Config
var StartTime = uint64(time.Now().Unix())
var nicks = make(map[string]string)

//HandleError needs to be implemented to be a valid WhatsApp handler
func (h *waHandler) HandleError(err error) {

    if e, ok := err.(*whatsapp.ErrConnectionFailed); ok {

        log.Printf("Connection failed, underlying error: %v", e.Err)
        log.Println("Waiting 30sec...")
        h.wait <- 30
        <-time.After(30 * time.Second)

        log.Println("Reconnecting...")
        err := h.c.Restore()
        if err != nil {
            log.Fatalf("Restore failed: %v", err)
        }
    }
}

// Implement HandleXXXMessage for any needed types
func (*waHandler) HandleTextMessage(m whatsapp.TextMessage) {

    if m.Info.Timestamp < StartTime {
        log.Printf("Skipping old message (%v) with timestamp %v\n", m.Text, m.Info.Timestamp)
        return
    }

    if m.Info.RemoteJid != cfg.groupid {
        log.Printf("RemoteJid %v does not match groupid %v, skipping\n", m.Info.RemoteJid, cfg.groupid)
        return
    }

    log.Printf("Timestamp: %v; ID: %v; Group: %v; Sender: %v; Text: %v\n",
        m.Info.Timestamp, m.Info.Id, m.Info.RemoteJid, *m.Info.Source.Participant, m.Text)

    var nick string
    if val, ok := nicks[*m.Info.Source.Participant]; ok {
        nick = val
    } else {
        nick = getAnon(*m.Info.Source.Participant, cfg.anon)
    }
    text := m.Text

    //scan for commands
    switch {
    case len(text) >= 5 && text[:5] == "!help":
        f, e := os.OpenFile(cfg.infile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
        if e != nil {
            log.Printf("Open infile failed: %v\n", e)
        }
        fmt.Fprintf(f, "This group is bridged to out of whatsapp. Your telephone number is obfuscated when relayed to these channels. You are now known as %v. Use the !setnick command to change this\n", nick)
        f.Close()
    case len(text) > 8 && text[:8] == "!setnick":
        parts := strings.Fields(m.Text)
        nnick := setNick(Nick{ phone: *m.Info.Source.Participant, nick: strings.Join(parts[1:], " "), }, cfg.nicks)
        if len(nnick) > 0 {
            f, e := os.OpenFile(cfg.infile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
            if e != nil {
                log.Printf("Open infile failed: %v\n", e)
            }
            fmt.Fprintf(f, "%v is now known as %v.\n", nick, nnick)
            f.Close()
            msg := cfg.prefix + "**" + nick + " is now known as " + nnick + "\n"
            relayToFile(msg, cfg.bridges)
            if len(cfg.teltoken) > 0 && len(cfg.telchat) > 0 {
                relayToTelegram(msg)
            }
        }
    default:
        //relay to bridges
        msg := cfg.prefix + nick + ": " + text + "\n"
        relayToFile(msg, cfg.bridges)
        if len(cfg.teltoken) > 0 && len(cfg.telchat) > 0 {
            relayToTelegram(msg)
        }
    }

}

// Implement HandleImageMessage
func (h *waHandler) HandleImageMessage(m whatsapp.ImageMessage) {

    if m.Info.Timestamp < StartTime {
        log.Printf("Skipping old message (%v) with timestamp %v\n")
        return
    }

    if m.Info.RemoteJid != cfg.groupid {
        log.Printf("RemoteJid %v does not match groupid %v, skipping\n", m.Info.RemoteJid, cfg.groupid)
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

    var nick string
    if val, ok := nicks[*m.Info.Source.Participant]; ok {
        nick = val
    } else {
        nick = getAnon(*m.Info.Source.Participant, cfg.anon)
    }
    text := "**" + nick + " sends an image: " + cfg.url + "/" + filename
    if len(m.Caption) > 0 {
        text += " with caption: " + m.Caption
    }

    //relay to bridges
    relayToFile(cfg.prefix + text + "\n", cfg.bridges)
    if len(cfg.teltoken) > 0 && len(cfg.telchat) > 0 {
        relayToTelegram(cfg.prefix + text + "\n")
    }
}

// Implement HandleDocumentMessage
func (h *waHandler) HandleDocumentMessage(m whatsapp.DocumentMessage) {

    if m.Info.Timestamp < StartTime {
        log.Printf("Skipping old message (%v) with timestamp %v\n")
        return
    }

    if m.Info.RemoteJid != cfg.groupid {
        log.Printf("RemoteJid %v does not match groupid %v, skipping\n", m.Info.RemoteJid, cfg.groupid)
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

    fsplit := strings.Split(m.FileName, ".")
    filename := fmt.Sprintf("%v.%v", m.Info.Id, fsplit[len(fsplit)-1])
    file, e := os.Create(cfg.attach + "/" + filename)
    defer file.Close()
    if e != nil {
        return
    }
    _, e = file.Write(data)
    if e != nil {
        return
    }
    log.Printf("%v %v\n\tDocument received, saved at: %v/%v\n", m.Info.Timestamp, m.Info.RemoteJid, cfg.attach, filename)

    var nick string
    if val, ok := nicks[*m.Info.Source.Participant]; ok {
        nick = val
    } else {
        nick = getAnon(*m.Info.Source.Participant, cfg.anon)
    }
    text := "**" + nick + " sends a document: " + cfg.url + "/" + filename

    //relay to bridges
    relayToFile(cfg.prefix + text + "\n", cfg.bridges)
    if len(cfg.teltoken) > 0 && len(cfg.telchat) > 0 {
        relayToTelegram(cfg.prefix + text + "\n")
    }
}

func (h *waHandler) HandleStickerMessage(m whatsapp.StickerMessage) {

    if m.Info.Timestamp < StartTime {
        log.Printf("Skipping old message (%v) with timestamp %v\n")
        return
    }

    if m.Info.RemoteJid != cfg.groupid {
        log.Printf("RemoteJid %v does not match groupid %v, skipping\n", m.Info.RemoteJid, cfg.groupid)
        return
    }

    fmt.Printf("Sticker: %+v\n", m)

    var nick string
    if val, ok := nicks[*m.Info.Source.Participant]; ok {
        nick = val
    } else {
        nick = getAnon(*m.Info.Source.Participant, cfg.anon)
    }
    text := "**" + nick + " sends a Sticker"

    //relay to bridges
    relayToFile(cfg.prefix + text + "\n", cfg.bridges)
    if len(cfg.teltoken) > 0 && len(cfg.telchat) > 0 {
        relayToTelegram(cfg.prefix + text + "\n")
    }
}

func main() {

    //get configuration
    var conf string
    flag.StringVar(&conf, "conf", "/etc/hermod.toml", "Path to TOML configuration")
    flag.Parse()
    fmt.Println("Toml file:", conf)

    cfg = getConfig(conf)

    //initialize Nick database
    nicks = readNicks(cfg.nicks)

    //create new WhatsApp connection
    wac, e := whatsapp.NewConn(5 * time.Second)
    if e != nil {
        log.Fatalf("error creating connection: %v\n", e)
    }

    //create buffered wait channel
    w := make(chan int64, 10)

    //Add handlers
    wac.AddHandler(&waHandler{wac,w})

    //login or restore
    if e := login(wac); e != nil {
        log.Fatalf("error logging in: %v\n", e)
    }

    //verify phone connectivity
    pong, e := wac.AdminTest()

    if !pong || e != nil {
        log.Fatalf("error pinging in: %v\n", e)
    }

    //start reading infile
    go infile(wac,w)

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

func relayToFile(msg string, bridges []string) {

    //relay to bridges
    for _, infile := range bridges {

        f, e := os.OpenFile(infile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
        if e != nil {
            continue
        }
        if _, e := f.Write([]byte(msg)); e != nil {
            log.Printf("Write to %v failed: %v\n", infile, e)
            continue
        }
        if e := f.Close(); e != nil {
            log.Println(e)
            continue
        }
    }
}

func relayToTelegram(msg string) {

    // relay to telegram
    telurl := "https://api.telegram.org/bot" + cfg.teltoken + "/sendMessage?chat_id=" + cfg.telchat + "&text="
    if _, e := http.Get(telurl + url.QueryEscape(msg)); e != nil {
        log.Printf("Telegram fail: %v\n", e)
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

