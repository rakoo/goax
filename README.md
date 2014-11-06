# Goax

A pure-go implementation of the Axolotl ratchet as defined in

  https://github.com/trevp/axolotl/wiki

and largely copied from @agl's Pond:

  https://github.com/agl/pond/tree/master/client/ratchet

The code includes a simple, stupid demo so you can test it yourself.

THIS IS PURELY EXPERIMENTAL, DON'T TRUST YOUR COMMUNICATIONS WITH IT

# How to use the code

In one window:

  ```shell
  > cd channelserver
  > go run main.go
  ```

In a 2nd and a 3rd window:

  ```shell
  > cd channelclient
  > go run main.go
  ```

  As explained, you will need to copy the json from the 2nd into the 3rd
  and vice-versa. The json will look like this:

  ```javascript
  {"idpub":"79bef716954c767dc641139977d52c57a452ce082d82d689e9cdbfac6810061e","dh":"a8047f6d57f9ef1e9958c53c331e667a06c8c7eeb0e419a4107c159ada8c6d5d","dh1":"59b9acbd0fd8a1fb1a883151a6155d3546e9edb7b03a01394a09193f08268c7d"}
  ```

In client windows, send messages with

  ```
  > m hello there !

  > m how are you ?
  ```

  You can send any number of messages before the other party reads them,
  because the Axolotl ratchet allows asynchrounous messages. Yay !

  To read messages:

  ```
  > g
  < [2014-11-06T22:21:04+01:00] hello there !
  ```
