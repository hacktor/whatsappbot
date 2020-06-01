package main

import (
    "encoding/gob"
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"

    qrcodeTerminal "github.com/Baozisoftware/qrcode-terminal-go"
    "github.com/Rhymen/go-whatsapp"
    "github.com/pelletier/go-toml"
)

type Config struct {
    groupid  string
    infile   string
    db       string
    attach   string
    url      string
    session  string
    sigurl   string
    teltoken string
    telchat  string
    telurl   string
    anon     string
    bridges  []string
}

var cfg Config

type waHandler struct {
    c *whatsapp.Conn
}

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
    log.Printf("Timestamp: %v; ID: %v; Group: %v; Sender: %v; Text: %v\n",
        m.Info.Timestamp, m.Info.Id, m.Info.RemoteJid, *m.Info.Source.Participant, m.Text)
}

func main() {

    //get configuration
    if t, e := toml.LoadFile("/etc/hermod.toml"); e != nil {
        log.Fatalf("error loading configuration: %v\n", e)
    } else {
        cfg = Config{
            groupid:  t.Get("whatsapp.groupid").(string),
            infile:   t.Get("whatsapp.infile").(string),
            db:       t.Get("whatsapp.db").(string),
            attach:   t.Get("whatsapp.attachments").(string),
            url:      t.Get("whatsapp.url").(string),
            session:  t.Get("whatsapp.session").(string),
            sigurl:   t.Get("signal.url").(string),
            teltoken: t.Get("telegram.token").(string),
            telchat:  t.Get("telegram.chat_id").(string),
            telurl:   t.Get("telegram.url").(string),
            anon:     t.Get("common.anon").(string),
            bridges:  []string{t.Get("irc.infile").(string), t.Get("signal.infile").(string), t.Get("matrix.infile").(string)},
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

