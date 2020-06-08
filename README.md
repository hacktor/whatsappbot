# Hermod Whatsapp Bot

[Hermod, or Hermóðr](https://en.wikipedia.org/wiki/Herm%C3%B3%C3%B0r) is a figure in Norse mythology,
often considered the messenger of the gods

This bot is used in a whatsapp group, relays messages to and from the group. For now it has an incoming infile and multiple configurable outgoing files (bridges) for relaying messages to other programs. In the near future, a socket server is planned to control the bot as well as a means to relay whatsapp messages over the network to other services. It's also possible to relay to a telegram group.

By default, a toml configuration file is read from /etc/hermod.toml

```toml
[common]
sendalarm       = "/home/hermod/bin/alarm"
anon            = "Anonymous"

[telegram]
chat_id         = "-1111111111111"
token           = "999999999:XXXXXXXXXXXXXXXXXXXXXXXXXXXX"

[whatsapp]
groupid         = "11111111111-1111111111@g.us"
infile          = "/home/hermod/log/towhatsapp.log"
nicks           = "/home/hermod/db/whatsapp.gob"
prefix          = "[whatsapp] "
attachments     = "/var/www/html/whatsapp"
session         = "/home/hermod/db/whatsappsession.gob"
url             = "/var/www/html/whatsapp"
bridges         = [
    "/home/hermod/log/fromwhatsapp.log",
    "/home/hermod/log/toanotherapp.log"
]

```
Only the [whatsapp.groupid] key is mandatory. All other keys have dummy defaults meaning they will largely be ignored. The bot listens in a whatsapp group and copies messages, prefixed with "[prefix] <anonymized>", where the whatsapp users' telephone number is replaced with a string (**common.anon** from the toml configuration if defined) and its last 4 numbers. If **whatsapp.prefix** is defined that will also be prefixed before relaying to the bridges.

The messages are then written to the defined bridge files, and can be picked up by other bots or applications.

## Configuration

Make a copy of the file hermod.toml.example to /etc/hermod.toml and change values
appropriately

The bot keeps a small binary gob database **whatsapp**->**nicks**, used for mapping whatsapp telephone numbers (in the whatsapp group) to nicknames. Members of the whatsapp group can set their nick by issuing the command:
```text
!setnick nickname
```
In the whatsapp group. The bot will update the mapping in the database and confirm this by saying:
```text
Anonymous-XXXX is now known as **nickname**
```
In all channels.

## Directories for attachments and urls

The photo's and attachments send by people in the whatsapp group are downloaded and placed in a directory. Use the **whatsapp-\>attachments** configuration option. Make sure this directory is shared over a HTTP webserver like apache and it is writeable by the webserver. Configure **whatsapp-\>url** to point to this same directory.

## Building the bot

You need a working go compiler and developers tools on your system. The Makefile can be used to compile a static binary for linux or windows:

```bash
~/src/git/whatsappbot$ make
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o whatsappbot whatsappbot.go nicks.go infile.go config.go
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o whatsappbot.exe whatsappbot.go nicks.go infile.go config.go
~/src/git/whatsappbot$
```

## Starting the bot

```bash
$ ./whatsappbot
Skipping old message ....
Skipping old message ....
....
2020/05/29 12:39:50 Seeked /home/hermod/log/towhatsapp.log - &{Offset:0 Whence:2}
```

To make the bot survive a crash or a reboot, you might want to add a line in your crontab:

```bash
@reboot screen -S whatsapp -d -m while true; do /home/hermod/bin/whatsappbot; done
```
