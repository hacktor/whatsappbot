package main

import (
    "log"
    "github.com/pelletier/go-toml"
)

type Config struct {
    groupid  string
    infile   string
    nicks    string
    attach   string
    url      string
    session  string
    teltoken string
    telchat  string
    prefix   string
    anon     string
    bridges  []string
}

func getConfig(c string) Config {

    t, e := toml.LoadFile(c)
    if e != nil {
        log.Fatalf("error loading configuration: %v\n", e)
    }
    cfg := Config{}

    if ! t.Has("whatsapp.groupid") {
        log.Fatalln("whatsapp.groupid undefined, cannot start")
    } else {
        cfg.groupid = t.Get("whatsapp.groupid").(string)
    }

    if ! t.Has("whatsapp.infile") {
        log.Println("whatsapp.infile undefined, using default /tmp/towhatsapp.log")
        cfg.infile = "/tmp/towhatsapp.log"
    } else {
        cfg.infile = t.Get("whatsapp.infile").(string)
    }

    if ! t.Has("whatsapp.attachments") {
        log.Println("whatsapp.attachments undefined, using default /tmp")
        cfg.attach = "/tmp"
    } else {
        cfg.attach = t.Get("whatsapp.attachments").(string)
    }

    if ! t.Has("whatsapp.url") {
        log.Println("whatsapp.url undefined, using default http://example.org/whatsapp")
        cfg.url = "http://example.org/whatsapp"
    } else {
        cfg.url = t.Get("whatsapp.url").(string)
    }

    if ! t.Has("whatsapp.session") {
        log.Println("whatsapp.session undefined, using default /tmp/whatsappsession.gob")
        cfg.session = "/tmp/whatsappsession.gob"
    } else {
        cfg.session = t.Get("whatsapp.session").(string)
    }

    if ! t.Has("whatsapp.nicks") {
        log.Println("whatsapp.nicks undefined, using default /tmp/whatsapp.gob")
        cfg.nicks = "/tmp/whatsapp.gob"
    } else {
        cfg.nicks = t.Get("whatsapp.nicks").(string)
    }

    if ! t.Has("telegram.token") {
        log.Println("telegram.url undefined")
        cfg.teltoken = ""
    } else {
        cfg.teltoken = t.Get("telegram.token").(string)
    }

    if ! t.Has("telegram.chat_id") {
        log.Println("telegram.chat_id undefined")
        cfg.telchat = ""
    } else {
        cfg.telchat = t.Get("telegram.chat_id").(string)
    }

    if ! t.Has("whatsapp.prefix") {
        log.Println("whatsapp.prefix undefined")
        cfg.prefix = ""
    } else {
        cfg.prefix = t.Get("whatsapp.prefix").(string)
    }

    if ! t.Has("common.bridges") {
        log.Println("common.bridges undefined")
        cfg.bridges = []string{}
    } else {
        cfg.bridges = t.GetArray("common.bridges").([]string)
    }

    if ! t.Has("common.anon") {
        log.Println("common.anon undefined")
        cfg.anon = "Anonymous"
    } else {
        cfg.anon = t.Get("common.anon").(string)
    }

    return cfg
}

