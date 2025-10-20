# Introduction to go-tor

## What This Software Does

go-tor is a program that helps you browse the internet privately and anonymously. It acts as a middleman between your computer and the websites you visit, routing your internet traffic through a network of volunteer computers around the world. This makes it very difficult for anyone to track what websites you're visiting or where you're located. Think of it like sending a letter through multiple post offices, each one only knowing where it came from and where it needs to go next, but none of them knowing the complete journey from you to the final destination.

## Table of Contents

- [How Everything Fits Together](#how-everything-fits-together)
- [The Repository Structure](#the-repository-structure)
  - [cmd - Command Line Programs](#cmd---command-line-programs)
  - [pkg - Package Library](#pkg---package-library)
  - [docs - Documentation](#docs---documentation)
  - [examples - Usage Examples](#examples---usage-examples)
  - [scripts - Build Automation](#scripts---build-automation)
- [How the Code Works Together](#how-the-code-works-together)
  - [Dependencies Map](#dependencies-map)
  - [Execution Flow](#execution-flow)
- [Key Concepts Explained](#key-concepts-explained)

## How Everything Fits Together

Imagine go-tor as a sophisticated post office system for your internet connections. When you want to visit a website privately, your request doesn't go directly there. Instead, it goes through three different "post offices" (called relays or nodes) in the Tor network. Each post office only knows about the one before it and the one after it, not the complete journey.

The go-tor software has several major parts working together:

1. **The Coordinator** - Manages the overall operation and keeps everything running smoothly
2. **The Route Planner** - Figures out which post offices (relays) to use for your journey
3. **The Security Team** - Encrypts your messages so each post office can only read its own instructions
4. **The Receptionist** - Takes your internet requests and packages them for the journey
5. **The Network Connector** - Actually talks to the post offices and sends your encrypted packages

All these parts communicate with each other to ensure your internet browsing stays private and anonymous.

## The Repository Structure

The code is organized like a well-structured library, with different sections for different purposes. Here's what each main folder contains and why:

### cmd - Command Line Programs

**Purpose**: This folder contains the actual programs you can run on your computer.

**Contains**: The main Tor client application that you launch when you want to start using Tor.

#### tor-client

This is the main program file. When you double-click or run this program from your terminal, it's like pressing the power button that starts the entire Tor privacy system. The program reads your settings, initializes all the components, and keeps everything running until you shut it down.

### pkg - Package Library

**Purpose**: This is the toolbox that contains all the specialized tools and components the main program uses. It's organized into separate compartments, each with a specific job.

**Contains**: Twenty different specialized components, each handling a specific aspect of connecting to and using the Tor network.

#### autoconfig

Think of this as the "first-time setup wizard." When you run go-tor for the first time, this component figures out where to store your data files, which ports to use, and other settings automatically. It's like when you plug in a new device and it configures itself without you having to make lots of technical decisions.

#### cell

In the Tor network, information is broken into small packages called "cells" (imagine them as standardized shipping boxes). This component knows how to pack information into these boxes and unpack them when they arrive. It ensures every box is the right size and properly labeled so the network can handle them efficiently.

#### circuit

A circuit is your private path through the Tor network, going through three different relays. This component is like a construction crew that builds, maintains, and tears down these paths. It keeps track of which relays you're using and makes sure the path stays working. If something goes wrong with one path, it builds a new one.

#### client

This is the orchestra conductor that brings all the other components together. It coordinates between the route planner, the network connections, the privacy features, and everything else. When you start the program, this component makes sure everyone knows their job and works together smoothly.

#### config

This component handles all the settings and preferences. It's like the control panel where you can adjust things like which port to use, where to store files, and how much information to log. It can read settings from a file or use smart defaults if you don't want to configure anything yourself.

#### connection

This component manages the actual internet connections to Tor relays. It's like a phone operator that establishes and maintains calls between your computer and the volunteer computers in the Tor network. It handles connection failures, retries, and makes sure the connections stay secure using encryption.

#### control

Some advanced programs need to communicate with go-tor to check its status or give it commands. This component is like a customer service desk that answers questions and accepts requests from other programs. It can tell you things like "a new circuit was built" or "data is flowing through the network."

#### crypto

This is the security vault. It contains all the tools for encrypting and decrypting information, verifying identities, and ensuring nobody can tamper with your data. It uses mathematical techniques to scramble your information so thoroughly that only the intended recipient can unscramble it.

#### directory

The Tor network has thousands of volunteer relay computers, and their information is stored in a directory. This component is like a librarian that fetches the latest list of available relays, their capabilities, and their trustworthiness ratings. It updates this information regularly so you always have current details about which relays you can use.

#### errors

When something goes wrong, this component helps the program understand what happened and how serious it is. It's like a diagnostic system in a car that categorizes problems (is this just a warning light or is the engine failing?) and provides clear explanations for troubleshooting.

#### health

This component continuously monitors the program to make sure everything is working correctly. It's like a doctor performing routine checkups, ensuring that circuits are being built, connections are working, and no component is struggling or failing.

#### logger

This is the program's journal or diary. Every significant event (starting up, building a circuit, encountering a problem) gets recorded here with timestamps and details. When troubleshooting problems, developers look at these logs to understand what the program was doing at any given time.

#### metrics

This component tracks statistics about the program's performance: how many circuits have been built, how fast data is flowing, how much memory is being used. It's like the dashboard in a car showing speed, fuel level, and engine temperature. These measurements help understand if the program is running efficiently.

#### onion

"Onion services" (also called hidden services) are special websites that only exist on the Tor network and have addresses ending in ".onion". This component handles the complex process of connecting to these services or hosting your own. It's named after the onion because, like an onion's layers, your data is wrapped in multiple layers of encryption.

#### path

When you need to send data through the Tor network, this component is the GPS navigator that chooses which three relays to use. It follows specific rules: pick a trusted "guard" relay you use consistently, pick a random middle relay, and pick an exit relay that can access your destination. This selection process is crucial for maintaining your privacy.

#### pool

Creating connections and circuits takes time and computer resources. This component is like a pool of pre-warmed cars waiting to take you somewhere. Instead of building everything from scratch each time, it maintains a collection of reusable resources (memory buffers, connections, partially-built circuits) so the program can respond faster to your requests.

#### protocol

This component speaks the Tor network's language. Just like people use specific phrases when they first meet ("Hello, my name is..."), this component knows the correct way to introduce your computer to a Tor relay, negotiate what features to use, and establish a secure conversation.

#### security

This is an extra layer of security review, like having a security guard double-check everything. It implements additional safety measures to ensure that the program follows best practices for protecting your privacy and handling sensitive information.

#### socks

SOCKS is a standard method that programs use to route their internet traffic through a proxy. This component runs a SOCKS server (specifically version 5 of the SOCKS protocol) that listens for connections from your web browser or other programs. When your browser wants to visit a website, it connects to this component, which then routes the request through the Tor network.

#### stream

Once a circuit is built, you might want to visit multiple websites through the same circuit. Each website connection is called a "stream." This component manages multiple simultaneous streams flowing through a single circuit, like how multiple phone calls can happen over the same internet connection. It keeps track of which data belongs to which website request.

### docs - Documentation

**Purpose**: This folder is like the instruction manual and reference library for the project.

**Contains**: Multiple documents explaining how the software works, how to use it, and how to develop it further.

The documentation includes guides for developers (DEVELOPMENT.md), explanations of the overall design (ARCHITECTURE.md), tutorials for users (TUTORIAL.md), and troubleshooting help (TROUBLESHOOTING.md). If you're new to the project or stuck on something, this is where you'd look for answers.

### examples - Usage Examples

**Purpose**: This folder contains working sample programs that show you how to use go-tor in different ways.

**Contains**: Over a dozen small programs demonstrating various features, from basic usage to advanced capabilities.

These examples are like cooking recipes - they show you step-by-step how to accomplish specific tasks. There's a "zero-config" example showing the simplest possible way to use go-tor, examples for advanced features like hosting onion services, and demonstrations of performance tuning. If you're building a program that uses go-tor, you'd start by looking at these examples.

### scripts - Build Automation

**Purpose**: Contains helper scripts that automate repetitive tasks during development and testing.

**Contains**: Shell scripts and other automation tools for building, testing, and maintaining the software.

These scripts are like kitchen gadgets that make cooking easier - they're not essential, but they save time and reduce mistakes when you're doing common tasks repeatedly.

## How the Code Works Together

### Dependencies Map

Understanding which components rely on which others helps you see the flow of information and responsibility:

**The Client Component** is at the top of the hierarchy. It relies on almost everything else:
- Calls upon the **Config Component** to load settings
- Uses the **Logger Component** to record events
- Relies on the **Directory Component** to get the list of available relays
- Depends on the **Path Component** to choose which relays to use
- Needs the **Circuit Component** to build and maintain paths through the network
- Uses the **SOCKS Component** to accept connections from your web browser
- May use the **Control Component** to communicate with other programs
- Relies on the **Metrics Component** to track performance
- Depends on the **Pool Component** for efficient resource management

**The Circuit Component** sits in the middle layer. It:
- Relies on the **Connection Component** to talk to relay computers
- Uses the **Crypto Component** to encrypt data at each hop
- Depends on the **Cell Component** to format messages correctly
- Calls upon the **Stream Component** to manage multiple simultaneous connections

**The Connection Component** is near the bottom. It:
- Uses the **Protocol Component** to speak the Tor network's language
- Relies on the **Crypto Component** for encryption
- Depends on basic networking capabilities provided by the Go programming language

**The SOCKS Component** is a gateway. It:
- Receives connections from your programs (web browsers, etc.)
- Sends traffic to the **Circuit Component** for routing
- Relies on the **Stream Component** to manage multiple connections

**Supporting Components** that many others depend on:
- **Logger** - Used by nearly every component to record what's happening
- **Config** - Referenced by most components to check settings
- **Crypto** - Called whenever encryption or security operations are needed
- **Errors** - Used by all components to report problems in a structured way

### Execution Flow

**When you start the program:**

1. **Initialization Phase** - The program first reads your settings (either from a configuration file you provided, command-line options, or smart defaults). It's like checking your preferences before starting a journey: which port should I use? Where should I store my data? How much logging do you want?

2. **Setting Up the Workspace** - The program creates or verifies its data directory exists. This is where it will store information about relay nodes, guard nodes (relays you use consistently for better security), and other persistent data. It's like making sure you have a desk drawer for your important documents.

3. **Starting the Logger** - Before doing anything complex, the program starts its logging system so it can record everything that happens next. This is like opening a journal before embarking on a trip.

4. **Fetching the Directory** - The program contacts directory servers to get the current list of Tor relays. These directory servers are run by trusted members of the Tor Project. The program receives information about thousands of relays: their addresses, capabilities, bandwidth, and trustworthiness ratings. This step is like getting an updated road map before starting your journey.

5. **Selecting Guard Nodes** - From all the available relays, the program selects a few "guard" relays that it will use consistently (usually for a month or so). Using consistent guards is important for security - it prevents an attacker from repeatedly forcing you through malicious relays. The program saves these guard selections so it can reuse them next time you run the program.

6. **Starting the SOCKS Server** - The program opens a port on your computer (usually 9050) and starts listening for connections from your web browser or other programs. It's like opening your front door and waiting for visitors.

7. **Building Initial Circuits** - Before you even make your first request, the program proactively builds some circuits through the Tor network. This is an optimization - having circuits ready to go means your first web request will be faster. It selects three relays (guard, middle, exit) for each circuit and establishes the encrypted connections through each hop.

**When you make a web request (like visiting a website):**

1. **Connection Received** - Your web browser connects to the SOCKS server and says "I want to visit example.com on port 443 (HTTPS)". The SOCKS component receives this request.

2. **Circuit Selection** - The SOCKS component asks the circuit manager for an appropriate circuit. The circuit manager looks at its available circuits and picks one whose exit relay can handle your request. If no suitable circuit exists, it builds a new one.

3. **Stream Creation** - A new stream is created within the chosen circuit. The stream component tells the circuit to open a connection to example.com through the exit relay. This involves sending special messages through the circuit's encrypted tunnel.

4. **Data Forwarding** - Once the stream is established, data starts flowing. When your browser sends data (like an HTTP request), the stream component packages it, the cell component formats it into Tor cells, the crypto component encrypts it layer-by-layer (once for each hop), and the connection component sends it to the first relay. Each relay removes one layer of encryption and forwards it to the next relay, until the exit relay sends your original request to example.com.

5. **Response Handling** - The website's response flows back through the same path in reverse. The exit relay receives it, forwards it through the circuit, and each relay adds a layer of encryption. When it reaches your computer, the crypto component removes all the layers, revealing the original response, which gets sent back to your browser through the SOCKS connection.

6. **Ongoing Operation** - This process continues for all your web requests. The circuit manager periodically checks circuit health, builds new circuits to replace old ones, and manages the pool of available paths. The metrics component tracks how much data is flowing. The logger records significant events.

**When you shut down the program:**

1. **Shutdown Signal** - You press Ctrl+C or close the program, which sends a shutdown signal.

2. **Graceful Cleanup** - The program enters graceful shutdown mode. It stops accepting new SOCKS connections, finishes processing any in-flight requests, and cleanly closes existing streams and circuits by informing the relays that you're disconnecting.

3. **Saving State** - Before exiting, the program saves important information (like your guard nodes) to disk so it can resume with the same configuration next time.

4. **Resource Release** - Finally, the program releases all its resources: closes network connections, frees memory, and terminates all background workers. It's like cleaning up your workspace before leaving.

## Key Concepts Explained

**Tor Network**: Think of the Tor network as a volunteer-run postal system for internet traffic. Instead of sending your letter (data) directly to its destination, you send it through three different volunteer post offices (relays). Each post office puts your letter in a new envelope addressed to the next post office, and only the last post office sees the final destination. This way, no single post office knows both where the letter came from and where it's going.

**Circuit**: A circuit is your current path through the Tor network. It consists of exactly three relays: a guard relay (your consistent entry point), a middle relay (chosen randomly), and an exit relay (chosen based on where you're going). Think of it as a specific route you take through three different neighborhoods to get to your destination. If anyone watched just one neighborhood, they'd only see you entering or leaving, not your complete journey.

**Cell**: In networking, information is broken into small chunks for transmission. Tor uses fixed-size chunks called cells (512 bytes of data plus some header information). This is like how letters have a standard envelope size - it makes processing more efficient and helps hide information about what you're sending (since all envelopes look the same size, regardless of contents).

**Relay**: A relay is a volunteer computer in the Tor network that forwards encrypted traffic. Imagine each relay as a helpful post office that receives sealed packages and forwards them to the next destination. The relay can't read the contents of your package (it's encrypted), but it knows where it came from and where it needs to go next.

**Guard Node**: For security reasons, you don't use a completely random relay as your first hop every time. Instead, you pick a few trusted guard relays and use them consistently for several weeks or months. This is like having a regular post office you always use for mailing letters - it reduces the chance of an attacker eventually forcing you through a malicious entry point.

**Stream**: Once you have a circuit established (your path through three relays), you can send multiple separate connections through that same circuit. Each connection is called a stream. It's like how you might drive the same route to town, but make stops at several different stores. Each store visit is a different stream, but they all use the same circuit (route).

**SOCKS Proxy**: SOCKS is a standardized method for programs to route their traffic through a middleman. When you configure your web browser to use a SOCKS proxy, the browser doesn't connect directly to websites. Instead, it connects to the SOCKS proxy and says "please connect me to example.com." The SOCKS proxy then makes the actual connection. In go-tor, the SOCKS component routes these connections through Tor circuits instead of connecting directly.

**Directory Server**: The Tor network needs a way to keep track of all available relays. Directory servers are trusted computers that maintain and distribute this information. They're like the information desks at a large mall - they maintain an updated map of all the stores (relays), what services they offer, and which ones are currently open. Your go-tor client contacts these directory servers periodically to get updated relay information.

**Onion Service** (Hidden Service): Normal websites have public addresses like "example.com" that anyone can look up. Onion services have special addresses ending in ".onion" that only work inside the Tor network. These services can provide anonymity for both the visitor (you) and the website itself. The website's location remains hidden because connections are made through rendezvous points in the Tor network. It's like two people meeting in a neutral location without either knowing where the other came from.

**Encryption Layers**: Your data gets encrypted three times (once for each relay in your circuit), like wrapping a gift in three boxes, one inside the other. Each relay can only open one box, revealing the next destination and another wrapped box. Only the final relay opens the innermost box and sees the original data. This "layered" encryption is why Tor is sometimes represented by an onion (which also has layers).

**Circuit Building**: When creating a new circuit, you don't simply ask three relays to form a path. Instead, you connect to the first relay, then ask it to connect to the second relay (sending this request encrypted so the first relay can't see which second relay you're asking for), then ask the second relay to connect to the third relay. Each step involves cryptographic negotiation to establish shared secrets that let you encrypt data for that hop. It's like building a tunnel one section at a time, making sure each section is secure before adding the next.

**Path Selection**: Choosing which three relays to use isn't random. The path selector follows specific rules: guards must have certain stability and bandwidth characteristics; you avoid using relays in the same family (run by the same operator); you consider geographic diversity; you select an exit relay that allows the type of traffic you need (some exits don't allow certain ports). These rules ensure good performance while maintaining strong privacy properties.

**Consensus Document**: Every hour, the directory servers vote and produce a consensus document - an agreed-upon list of all active Tor relays and their properties. When your client fetches the directory, it's downloading this consensus document. Think of it as the directory servers holding a meeting every hour, comparing their notes about which relays are online and trustworthy, and publishing their collective agreement.

**Zero-Configuration Mode**: One of go-tor's features is that it can run without requiring you to configure anything. When you start it without providing a configuration file, it automatically detects an appropriate data directory for your operating system, finds available network ports, and uses sensible defaults for all settings. It's like a "plug and play" feature - it figures out reasonable settings on its own so you don't need to be a technical expert to use it.

