# Chaingo Blockchain

![Go Version](https://img.shields.io/badge/go-%3E%3D1.18-blue)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

Chaingo is a modular blockchain implementation in Go featuring dual consensus mechanisms (PoW/PoA), peer synchronization, and cryptographic transaction handling.

## Features

### Consensus Mechanisms
- **Proof of Work (PoW)**: CPU-based mining with adjustable difficulty
- **Proof of Authority (PoA)**: Round-robin validator selection
  - Deterministic leader election
  - Validator rotation

### Network Layer
- Peer-to-peer communication
- Automatic node discovery
- Block/transaction propagation
- Mempool synchronization

### Cryptography
- ECDSA digital signatures
- Keccak-256 hashing
- Address generation from public keys


## Admin CLI Tools
Chaingo provides command-line utilities for key management and transaction operations:

```bash
admin <command> [flags]

Commands:
  genkey    Generate ECDSA private key
  send      Send signed transaction

```


### Key Generation
Generate ECDSA private key
```bash
Usage:
  admin genkey -dir=<directory> -account=<name>

Flags:
  -dir string      Directory to store the generated key (required)
  -account string  Name of the account associated with the key (required)    Send signed transaction

```
### Signed Transaction
Send signed transaction
```bash
Usage:
  admin send -account=<name> -dir=<keys_dir> -url=<node> -nonce=<nonce> 
             -from=<from_id> -to=<to_id> -value=<value> -tip=<tip> [-data=<hex_data>]

Flags:
  -account string  Account name to use for signing (required)
  -dir string      Directory containing private keys (default "block")
  -url string      URL of the node (default "http://localhost:8000")
  -nonce uint64    Nonce for the transaction (required)
  -from string     Sender account ID (required)
  -to string       Recipient account ID (required)
  -value uint64    Value to send (required)
  -tip uint64      Tip to include (required)
  -data string     Hex-encoded data to include in transaction

```

## Getting Started

### Prerequisites
- Go 1.18+


### Installation
```bash
git clone https://github.com/hamidoujand/chaingo
cd chaingo

# run first node
make run  

# run second node 
make run2

#run third node if you want
make run3

#there are some wallet addresses inside makefile for testing the network and
#also commands such as:

#run first because of the nonce order check
make load
#run second because of the nonce order check
make load2

#blocks will be places inside of <block> directory categorized by each miner.
```