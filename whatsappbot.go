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
)

type waHandler struct {
    c *whatsapp.Conn
}

type config struct {
    groupid  string
    infile   string
    attach   string
    ircfile  string
    sigfile  string
    matfile  string
    teltoken string
    telchat  string
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
func (*waHandler) HandleTextMessage(message whatsapp.TextMessage) {
    if message.Info.Timestamp < StartTime {
        fmt.Printf("Skipping old message (%v) with timestamp %v\n", message.Text, message.Info.Timestamp)
        return
    }
    fmt.Printf("Timestamp: %v\nID: %v\nRemoteID: %v\nMsgID: %v\nText:\t%v\n", message.Info.Timestamp, message.Info.Id, message.Info.RemoteJid, message.ContextInfo.QuotedMessageID, message.Text)
}

//Example for media handling. Video, Audio, Document are also possible in the same way
func (h *waHandler) HandleImageMessage(message whatsapp.ImageMessage) {
    if message.Info.Timestamp < StartTime {
        fmt.Printf("Skipping old message (%v) with timestamp %v\n")
        return
    }
    data, err := message.Download()
    if err != nil {
        if err != whatsapp.ErrMediaDownloadFailedWith410 && err != whatsapp.ErrMediaDownloadFailedWith404 {
            return
        }
        if _, err = h.c.LoadMediaInfo(message.Info.RemoteJid, message.Info.Id, strconv.FormatBool(message.Info.FromMe)); err == nil {
            data, err = message.Download()
            if err != nil {
                return
            }
        }
    }

    filename := fmt.Sprintf("%v/%v.%v", os.TempDir(), message.Info.Id, strings.Split(message.Type, "/")[1])
    file, err := os.Create(filename)
    defer file.Close()
    if err != nil {
        return
    }
    _, err = file.Write(data)
    if err != nil {
        return
    }
    log.Printf("%v %v\n\timage received, saved at:%v\n", message.Info.Timestamp, message.Info.RemoteJid, filename)
}

func main() {

    //get configuration
    if tree, err := toml.LoadFile("/etc/hermod.toml"); err != nil {
        log.Fatalf("error loading configuration: %v\n", err)
    } else {
        cfg.groupid = tree.Get("whatsapp.groupid").(string)
        cfg.infile = tree.Get("whatsapp.infile").(string)
        cfg.attach = tree.Get("whatsapp.attachments").(string)
        cfg.ircfile = tree.Get("irc.infile").(string)
        cfg.sigfile = tree.Get("signal.infile").(string)
        cfg.matfile = tree.Get("matrix.infile").(string)
        cfg.teltoken = tree.Get("telegram.token").(string)
        cfg.telchat = tree.Get("telegram.chat_id").(string)
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

func infile(wac *whatsapp.Conn) {

    // keep a tail on the infile
    if t, err := tail.TailFile(cfg.infile, tail.Config{Follow: true, ReOpen: true}); err != nil {

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
                fmt.Fprintf(os.Stderr, "error sending message: %v", err)
                os.Exit(1)
            } else {
                fmt.Println("Message Sent -> ID : " + msgId)
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
    file, err := os.Open(os.TempDir() + "/whatsappSession.gob")
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
    file, err := os.Create(os.TempDir() + "/whatsappSession.gob")
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

