package main

import (
    "log"
    "strings"
    "database/sql"
    _ "github.com/mattn/go-sqlite3"
)

type Nick struct {
    phone    string
    nick     string
}

func initNicks(sqldb string) {

    //open database
    db, e := sql.Open("sqlite3", sqldb)
    if e != nil {
        log.Fatalf("Error opening db: %v\n", e)
    }
    defer db.Close()

    rows, e := db.Query("SELECT phone, nick FROM alias")
    if e != nil {
        log.Fatalf("Query failed: %v\n", e)
    }
    defer rows.Close()

    for rows.Next() {

        var phone string
        var nick string

        e = rows.Scan(&phone, &nick)
        if e != nil {
            log.Printf("rows.Scan failed: %v\n", e)
            continue
        }
        nicks[phone] = nick
    }
}

func setNick(n Nick, sqldb string) string {

    //open database
    db, e := sql.Open("sqlite3", sqldb)
    if e != nil {
        log.Printf("Error opening db: %v\n", e)
        return ""
    }
    defer db.Close()

    if len(n.phone) > 0 && len(n.nick) > 0 {

        // new nick, insert

        nicks[n.phone] = n.nick
        stmt, e := db.Prepare("REPLACE INTO alias (phone,nick) values (?,?)")
        if e != nil {
            log.Printf("Prepare failed on db %v: %v\n", sqldb, e)
            return ""
        }
        defer stmt.Close()

        _, e = stmt.Exec(n.phone, n.nick)
        if e != nil {
            log.Printf("Replace nick failed: %v\n", e)
            return ""
        }
        return n.nick
    }

    // return empty nick 'cause idunno
    return ""
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

