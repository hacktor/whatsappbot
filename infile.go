package main

import (
    "fmt"
    "log"
    "os"
    "path"
    "strings"

    "github.com/Rhymen/go-whatsapp/binary/proto"
    "github.com/Rhymen/go-whatsapp"
    "github.com/hpcloud/tail"
)

func infile(wac *whatsapp.Conn) {

    // keep a tail on the infile
    loc := &tail.SeekInfo{Offset: 0, Whence: os.SEEK_END}
    if t, e := tail.TailFile(cfg.infile, tail.Config{Follow: true, ReOpen: true, Location: loc}); e == nil {

        for line := range t.Lines {

            text := line.Text
            fmt.Println(text)

            if len(text) > 5 && text[:5] == "FILE:" {

                // Get file info and upload
                parts := strings.Fields(text)
                if len(parts) < 2 {
                    log.Println("Too few parts in FILE line")
                    continue
                }
                info := strings.Split(parts[0], ":")

                if len(info) < 4 {
                    log.Println("FILE info is garbage")
                    continue
                }

                img, e := os.Open(info[3])
                if e != nil {
                    log.Printf("Failed to open file %v: %v\n", info[3], e)
                    continue
                }

                msg := whatsapp.ImageMessage{
                    Info: whatsapp.MessageInfo{
                        RemoteJid: cfg.groupid,
                    },
                    Type:    info[2],
                    Caption: strings.Join(parts[1:], " "),
                    Content: img,
                }

                // debug
                fmt.Printf("%+v\n", msg)

                msgId, e := wac.Send(msg)
                if e != nil {

                    // upload failed; fallthrough as link
                    log.Printf("error sending message: %v", e)
                    var link string

                    switch info[1] {
                    case "TEL":
                        link = cfg.telurl + "/" + path.Base(info[3])
                    case "SIG":
                        link = cfg.sigurl + "/" + path.Base(info[3])
                    default:
                        continue
                    }
                    text = strings.Join(parts[1:], " ") + " ( " + link + " )\n"

                } else {

                    // upload succeeded; go to next line
                    fmt.Println("Image Sent -> ID : "+msgId)
                    continue

                }
            }

            prevMessage := "?"
            quotedMessage := proto.Message{
                Conversation: &prevMessage,
            }

            ContextInfo := whatsapp.ContextInfo{
                QuotedMessage:   &quotedMessage,
                QuotedMessageID: "",
                Participant:     "",
            }

            msg := whatsapp.TextMessage{
                Info: whatsapp.MessageInfo{
                    RemoteJid: cfg.groupid,
                },
                ContextInfo: ContextInfo,
                Text:        text,
            }

            if msgId, e := wac.Send(msg); e != nil {
                log.Printf("error sending message: %v\n", e)
            } else {
                fmt.Printf("Message Sent -> ID : %v\n", msgId)
            }

        }

    } else {
        fmt.Printf("Error in tail file: %v\n", e)
    }
}

