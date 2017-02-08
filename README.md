# Goax

A pure-go implementation of the Signal protocol as defined in

https://en.wikipedia.org/wiki/Signal_Protocol

The actual implementation was written by @agl over here:

  https://github.com/agl/pond/tree/master/client/ratchet

Any credits go to him. This repo just extracts all pond-specific parts
to make the protocol usable outside of it.

As such, the usual warning:

THIS IS PURELY EXPERIMENTAL, DON'T TRUST YOUR COMMUNICATIONS WITH IT

# What it is

Goax is an attempt at using the Signal protocol from the command line.
The goal is to have a UX as simple as possible so that making mistakes
is hard. It doesn't do any network communication, that's the part where
you, the user, interact by copy-pasting goax's inputs and outputs
through the medium of your choice.

# How to use

goax is an extremely simple attempt at Forward Secure communications on
the command line. All it does is print armor-encoded blocks of text that
you copy-paste into the medium of your choice (email, IM, ...)

The first thing to do is to build the binary and put it in its own
folder

```shell
$ go build
$ cp goax /tmp/comms
$ cd /tmp/comms
```

Once there, it will create some files in the directory. Don't bother
about them.

Now that it's there you will want to run it, just to see what it does

```shell
$ ./goax
Need an action: one of mykey, send or receive
```

Let's see what our key is:

```shell
$ ./goax mykey
2JC2HDtUfBwxMGq4Bkj1DcAiGB2WQ3eByJ9jhyukDMob
```

Of course your key will differ. It is automatically created if it
doesn't exist (it's just a file in the current directory). Run it again;
the key should be the same.
This is your *identity key*. It uniquely identifies this instance of
goax. Delete the `key` file and you have another instance with another
identity; run goax from another directory and you have another instance
with another identity.

Now that we have an identity, we probably want to send some message to
someone. The first step is to try to send them something. Let's suppose
they are named Barry:

```shell
$ ./goax send barry
No ratchet for barry, please send this to the peer and "receive" what they send you back

-----BEGIN KEY EXCHANGE MATERIAL-----

eyJpZHB1YiI6IjEzNDMwNWZiNmRlMmU3YTc1NmU5NmRiODI0YmRkYjNkMzA4MTE1
OGRhNzkwZjZiYjUwZGZjZmM3YTI4MDYxNmUiLCJkaCI6ImU1YTYyNDU4ZjJmYWQ2
YTdkNWI2Y2ZkYjI1NWNhZDViNzg4YzVlMDNiNWYxMWY1NWFhYWRjYzk0NTY4Mjc2
MjUiLCJkaDEiOiI1MmFmOGYxNTRiZDk0ZDExNGE1ZjljMWIxMWQxOTMxYTU5Mjhm
Y2MwODY2NDE2NzczZGIwNGRkYmFhNGVmMzBhIn0K
=Xccr
-----END KEY EXCHANGE MATERIAL-----
```

What happened is that since it is the very first time that goax hears
about a "barry", it will create a ratchet (which is some internal state)
for them, and create a *key exchange material*. The details of what it
is aren't important; all you have to do is copy-paste the block
(including the -----BEGIN KEY EXCHANGE MATERIAL----- and
 -----END KEY EXCHANGE MATERIAL---- lines) and send it to barry. This
material serves to make a handshake with them; you have to send your
part to them, and they have to send their part to you.

Note that the argument `barry` is just a string; goax has no idea what
it means. It could very well be an email address (b@rry.com) or a
twitter handle (@rryb) or an ICQ handle. Every "recipient" is considered
a different conversation between your identity key and their identity
key.

We have sent them the block, and they have done the same on their side
and sent us their block. Here's how we finish the handshake (they should
do the same on their side as well)

```shell
$ ./goax receive barry
Please paste in the message; when done, hit Ctrl-D

-----BEGIN KEY EXCHANGE MATERIAL-----

eyJpZHB1YiI6IjEzNDMwNWZiNmRlMmU3YTc1NmU5NmRiODI0YmRkYjNkMzA4MTE1
OGRhNzkwZjZiYjUwZGZjZmM3YTI4MDYxNmUiLCJkaCI6Ijc3N2FhNWY0ZmI1NmRl
NTQ4YzI4ZDhmNDIxMjQ3ZDg3YjhkNWI0ZDhiN2I4NjY5NTRlNGZhNTVjOWQ5YjZj
NWMiLCJkaDEiOiJlZTQ4NzYyNDA3NmJmN2JlZmQwYmJiZjkwYWY3MGZjNDlhOGI2
NjA4MmU0Zjg4ODAzZTExZTNkMWQ2YzlhMjIxIn0K
=I2pa
-----END KEY EXCHANGE MATERIAL-----
^D
$
```

The command didn't throw anything at us; the handshake is done and we
can now start sending messages ! (Hit Ctrl-D on an empty line when you
are done writing your message)

```shell
$ ./goax send barry

Hello from goax !
^D
-----BEGIN GOAX ENCRYPTED MESSAGE-----

PvEwrQH8CFR9URIRX63SQtBA/BxoAIze/Higmkf54FYswgy5YzYIbiz6/n54dy/o
aLcVeAz2HA+5rReJHeVU5iNNd06BMlPzFxJCp21ssgiiVgSivPHb1/2mv8KMaqK8
cATkOxjryH6xEXmCRWPKTVJVZre2MwgiETdlJkqVwZjcnD7nJogXx2uq
=KNWM
-----END GOAX ENCRYPTED MESSAGE-----
```

This new block type is the actual encrypted message; send that to barry,
and they can read your message:

```shell
# From barry's shell
barry$ ./goax receive anon
Please paste in the message; when done, hit Ctrl-D

-----BEGIN GOAX ENCRYPTED MESSAGE-----

PvEwrQH8CFR9URIRX63SQtBA/BxoAIze/Higmkf54FYswgy5YzYIbiz6/n54dy/o
aLcVeAz2HA+5rReJHeVU5iNNd06BMlPzFxJCp21ssgiiVgSivPHb1/2mv8KMaqK8
cATkOxjryH6xEXmCRWPKTVJVZre2MwgiETdlJkqVwZjcnD7nJogXx2uq
=KNWM
-----END GOAX ENCRYPTED MESSAGE-----
^D

Hello from goax !

barry $
```

Happy communicating !

And remember: goax hasn't been audited or analyzed by any competent
cryptographer mind and probably contains multiple issues. Most notably
there is no way for a user to verify the identity of a peer. Don't
expect it to save your life.

# Alternative flow: sending messages before receiving any of them

The Signal protocol handshake has been built with asynchronicity in
mind. If you read carefully the previous section, a handshake doesn't
need both peers to be actually discussing, they don't need to be on at
the same time. This means that it is perfectly fine to receive barry's
key exchange material, complete the handshake on our side, and start
sending messages straight away !

```shell
$ ./goax receive barry
Please paste in the message; when done, hit Ctrl-D

-----BEGIN KEY EXCHANGE MATERIAL-----

eyJpZHB1YiI6IjEzNDMwNWZiNmRlMmU3YTc1NmU5NmRiODI0YmRkYjNkMzA4MTE1
OGRhNzkwZjZiYjUwZGZjZmM3YTI4MDYxNmUiLCJkaCI6ImY1NjU1MGU4MDYxYWE5
ZmNhM2QzM2UwNzYyMzI1ZWRhNDNhZGU2NDNhYzNlY2M3NWNiNGRkOTg3ZTEyMGFj
NDQiLCJkaDEiOiI0NDEyZDAyMTFhNDNiNDY5NjhlMTQ1MDQxZWZkZWY2ZDAyM2Jj
ZWM1ODAxMjA5NzFlMjc3ZWU1ODU3MmJjZTJiIn0K
=ossT
-----END KEY EXCHANGE MATERIAL-----
No ratchet for barry, creating one.

```

In this particular case, we didn't even know that barry existed; maybe
they wanted to talk to us ? In any case, the handshake is complete *on
our side*, so we can start sending messages straight away:

```shell
./goax send barry

Happy to hear from you !
^D
-----BEGIN KEY EXCHANGE MATERIAL-----

eyJpZHB1YiI6IjEzNDMwNWZiNmRlMmU3YTc1NmU5NmRiODI0YmRkYjNkMzA4MTE1
OGRhNzkwZjZiYjUwZGZjZmM3YTI4MDYxNmUiLCJkaCI6ImFmZTY1YjRhYWQzNzkw
OWY0MzQ3MGE3NDFlN2UyOWY1YjBlM2Q1NjViMjExZGUwNzVmODJiZGZiNTAwZTU4
NWMiLCJkaDEiOiJlODE0NTNkMDZlZWYxOTE1MjlkMGM1ZjcxYjgwNDRjYzA0NTc4
ZjliM2U5MjBkZWMzMDhiN2RkM2JkYzdlNDI2In0K
=QhHJ
-----END KEY EXCHANGE MATERIAL-----
-----BEGIN GOAX ENCRYPTED MESSAGE-----

ERXsufvBbGhICLZt5CBXrTZsRIKvVt0TKTRi05erz6aRkgIS/a9OYzvUm5wk40To
Payo6OYQOx8b1Hkqee4iNCzeiE+tEMVXBZEtI+AuZtNUIolRDtpI9vudH2XgYgkR
4hr9WnYbtCB1mOMvqHh0OA6LKd2aV3QDzmbC/7BknzBCEUTHLMA0jzGuV410j0VU
NA==
=w59U
-----END GOAX ENCRYPTED MESSAGE-----

$ 
```

Since we're not sure that barry has finished the handshake on their
side, it is safe to send them key exchange material again. Just copy
paste the 2 blocks and send them; barry will `receive` all of it and
goax will make what is necessary.

At a later time, when barry sends us a message and we successfully
decrypt it, we have 100% assurance that they have finished the handshake
on their side; goax won't output the key exchange material anymore.
