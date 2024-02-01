# Golang URL Middleware

A simple bit of middleware to sit between apps like browsers that can trigger URLs and local apps that you might want to call via a custom URL scheme.

The intended use of this is to allow links like [ping://8.8.8.8](ping://8.8.8.8) on a webpage to trigger the relevant native app.

Please note the following that this app is early days and has only been tested in a Windows environment. I can't see why Linux wouldn't work but I'm unsure how you would configure it.

Usage:  
`gum [options] <URL>`

Options:  
  [CURRENTLY ARE NONE]

Installation:  
On a Windows computer you can install it by roughly doing the following:
- Create a "GUM" folder in "C:\Program Files"
- Copy "gum.exe" into that folder
- Copy your config file into that folder making sure it is named "configuration.json"
- Repurpose one of the reg files in the misc folder of this repo to direct the protocol you want handled to the executable
- Test and troubleshoot until you get it working