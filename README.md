# Goax

A pure-go implementation of the Axolotl ratchet as defined in

  https://github.com/trevp/axolotl/wiki

The actual implementation was written by @agl over here:

  https://github.com/agl/pond/tree/master/client/ratchet

Any credits go to him. This repo just extracts all pond-specific parts
to make Axolotl usable outside of it.

As such, the usual warning:

THIS IS PURELY EXPERIMENTAL, DON'T TRUST YOUR COMMUNICATIONS WITH IT

The code includes a simple, stupid demo so you can test it yourself.

# How to use the example

The example in `example/cui` is a curses-like xmpp client. You must have
an xmpp account somewhere. If you don't there are many public providers
out there happy to host you: check out [this page](https://xmpp.net/directory.php)

Once you have an account, you also need contacts to talk to who also use
this application. You can always add me (rakoo@otokar.looc2011.eu)
  through "standard" xmpp clients, after which we can discuss through
  goax protocol. An echobot will surely be spawned someday.


To use the application:

1. Compile the code
2. Create a file called `config.json` next to the executable. It must
   contain the following data:

  ```json
  {
    "jid": "myjid@mydomain.com",
    "password": "mypassword",
    "ServerCertificateSHA256": "<base64 certificate of server>"
  }
  ```

3. Run the application, you are now in a curses-like application. On the
   left are all your contacts that can speak goax. On the right is the
   discussion window. In the bottom is your input.

4. To go to the contacts window, hit Ctrl-Space. You can select a
   contact with enter, or go back to input region with Ctrl-Space.

5. Once in the input region, you send messages by typing them and
   hitting Enter.

6. To leave the application, type `/quit`.
