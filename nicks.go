package main

import (
    "encoding/gob"
    "log"
    "strings"
    "os"
)

type Nick struct {
    phone    string
    nick     string
}

func setNick(n Nick, nickmap string) string {

    if len(n.phone) > 0 && len(n.nick) > 0 {

        // new nick, insert

        nicks[n.phone] = n.nick
        e := writeNicks(nicks, nickmap)
        if e != nil {
            log.Printf("Saving nicks to gob %v failed: %v\n", nickmap, e)
            return ""
        }
        return n.nick
    }

    // return empty nick 'cause idunno
    return ""
}

func getNick(participant string) string {

    var nick string
    if val, ok := nicks[participant]; ok {
        nick = val
    } else {
        nick = getAnon(participant, cfg.anon)
    }
    return nick
}

func getAnon(sender string, anon string) string {

    //anonymize telephone number
    sender = strings.Split(sender,"@")[0]
    if len(sender) > 8 {
        return anon + "-" + sender[7:]
    } else {
        return "Anonymous"
    }
}

func readNicks(nickgob string) (map[string]string) {
    nicks := make(map[string]string)
    file, e := os.Open(nickgob)
    if e != nil {
        return nicks
    }
    defer file.Close()
    decoder := gob.NewDecoder(file)
    e = decoder.Decode(&nicks)
    if e != nil {
        return nicks
    }
    return nicks
}

func writeNicks(nicks map[string]string, nickgob string) error {
    file, e := os.Create(nickgob)
    if e != nil {
        return e
    }
    defer file.Close()
    encoder := gob.NewEncoder(file)
    e = encoder.Encode(nicks)
    return e
}

