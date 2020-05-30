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

func generateNickGetSet() func(Nick) string {

    //this is the closure
    var nicks = make(map[string]string)

    //open database
    db, e := sql.Open("sqlite3", "test.db")
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

    return func(n Nick) string {

        //open database
        db, e := sql.Open("sqlite3", "test.db")
        if e != nil {
            log.Fatalf("Error opening db: %v\n", e)
        }
        defer db.Close()

        if len(n.phone) > 0 && len(n.nick) > 0 {

            // new nick, insert

            nicks[n.phone] = n.nick
            stmt, e := db.Prepare("REPLACE INTO alias (phone,nick) values (?,?)")
            defer stmt.Close()

            if e != nil {
                log.Printf("Prepare failed: %v\n", e)
                return ""
            }
            defer stmt.Close()

            _, e = stmt.Exec(n.phone, n.nick)
            if e != nil {
                log.Printf("Insert nick failed: %v\n", e)
            }
            return ""
        } else if len(n.phone) > 0 {

            // existing nick?
            if nick, ok := nicks[n.phone]; ok {

                return nick

            } else {

                return getAnon(n.phone)

            }
        }

        // return empty nick 'cause idunno
        return ""

    } // end generated function
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

