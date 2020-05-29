# Whatsapp Bot

[Hermod, or Hermóðr](https://en.wikipedia.org/wiki/Herm%C3%B3%C3%B0r) is a figure in Norse mythology,
often considered the messenger of the gods

This bot is used in tandem with the [Signal IRC Telegram Matrix Gateway](https://github.com/Piratenpartij/signal-irc-telegram-gateway). Configurable with the same toml configuration file. The relevant sections of this configuration file are as follows:

```toml
[common]
sendalarm       = "/home/hermod/bin/alarm"
anon            = "Anonymous"

[irc]
infile          = "/home/hermod/log/toirc.log"

[matrix]
infile          = "/home/hermod/log/tomatrix.log"

[signal]
infile          = "/home/hermod/log/tosignal.log"

[telegram]
chat_id         = "-1111111111111"
token           = "999999999:XXXXXXXXXXXXXXXXXXXXXXXXXXXX"

[whatsapp]
groupid         = "11111111111-1111111111@g.us"
infile          = "/home/hermod/log/towhatsapp.log"
db              = "/home/hermod/db/whatsapp.db"
debug           = "/home/hermod/log/whatsapp.debug"
attachments     = "/var/www/html/whatsapp"
session         = "/home/hermod/db/whatsappsession.gob"

```

## Configuration

Make a copy of the file hermod.toml.example to /etc/hermod.toml and change values
appropriately

The bot keeps a small sqlite database **whatsapp**->**db**, used for mapping whatsapp telephone numbers (in the whatsapp group) to nicknames. Members of the whatsapp group can set their nick by issuing the command:
```text
!setnick nickname
```
In the whatsapp group. The bot will update the mapping in the database and confirm this by saying:
```text
Anonymous-XXXX is now known as nickname
```
In all channels.

You need to create an sqlite database file:
```sql
sqlite> CREATE TABLE alias (phone text unique not null, nick text);
```

## Directories for attachments and urls

The photo's and attachments send by people in the whatsapp group are downloaded and placed in a directory. Use the **whatsapp-\>attachments** configuration option. Make sure this directory is shared over a HTTP webserver like apache and it is writeable by the webserver. Configure **whatsapp-\>url** to point to this same directory.

Verify permissions on the **irc/signal/matrix/whatsapp-\>infile** files. They should be writable by the user running the scripts and also by the webserver that is executing the telegram webHook. Then you can start the bot.

```bash
$ ./whatsappbot
Skipping old message ....
Skipping old message ....
....
2020/05/29 12:39:50 Seeked /home/hermod/log/towhatsapp.log - &{Offset:0 Whence:2}
```

To make the bot survive a crash or a reboot, you might want to add a line in your crontab:

```bash
@reboot screen -S matrix -d -m while true; do /home/hermod/bin/whatsappbot; done
```
