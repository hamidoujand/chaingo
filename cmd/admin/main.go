package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/hamidoujand/chaingo/database"
)

func printUsage() {
	const usage = `Admin CLI - Blockchain management tool

Usage:
  admin <command> [flags]

Commands:
  genkey    Generate ECDSA private key
  send      Send signed transaction

Use "admin <command> -help" for more information about a command
`
	fmt.Print(usage)
}

func printGenkeyUsage() {
	const usage = `Generate ECDSA private key

Usage:
  admin genkey -dir=<directory> -account=<name>

Flags:
  -dir string      Directory to store the generated key (required)
  -account string  Name of the account associated with the key (required)
`
	fmt.Print(usage)
}

func printSendUsage() {
	const usage = `Send signed transaction

Usage:
  admin send -account=<name> -dir=<keys_dir> -url=<node> -nonce=<nonce> 
             -from=<from_id> -to=<to_id> -value=<value> -tip=<tip> [-data=<hex_data>]

Flags:
  -account string  Account name to use for signing (default "hamid")
  -dir string      Directory containing private keys (default "block")
  -url string      URL of the node (default "http://localhost:8000")
  -nonce uint64    Nonce for the transaction (required)
  -from string     Sender account ID (required)
  -to string       Recipient account ID (required)
  -value uint64    Value to send (required)
  -tip uint64      Tip to include (required)
  -data string     Hex-encoded data to include in transaction
`
	fmt.Print(usage)
}

//==============================================================================

// byteSliceFlag is a custom flag used to store byte slices.
type byteSliceFlag []byte

func (b *byteSliceFlag) String() string {
	return hex.EncodeToString(*b)
}

func (b *byteSliceFlag) Set(value string) error {
	decode, err := hex.DecodeString(value)
	if err != nil {
		return fmt.Errorf("decodeString: %w", err)
	}
	*b = decode
	return nil
}

//==============================================================================

func main() {
	if err := run(); err != nil {
		printUsage()

		if len(os.Args) > 1 {
			switch os.Args[1] {
			case "genkey":
				printGenkeyUsage()
			case "send":
				printSendUsage()
			}
		}
		fmt.Printf("\nError: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("subcommand needs to be passed")
	}

	switch os.Args[1] {
	case "genkey":
		return handleGenerateKey()

	case "send":
		return handleSendTransaction()

	default:
		return fmt.Errorf("unknown command %q", os.Args[1])
	}

}

func handleGenerateKey() error {
	genkeyCommand := flag.NewFlagSet("genkey", flag.ExitOnError)
	keyDir := genkeyCommand.String("dir", "", "directory to put the generated key.")
	accountName := genkeyCommand.String("account", "", "name of the account associate with the key.")

	//custom usage
	genkeyCommand.Usage = func() {
		printGenkeyUsage()
	}

	genkeyCommand.Parse(os.Args[2:])

	if *keyDir == "" {
		return fmt.Errorf("<dir> is a required argument")
	}

	if *accountName == "" {
		return fmt.Errorf("<account> is a required argument")
	}

	return generateKey(*keyDir, *accountName)
}

func handleSendTransaction() error {
	sendCommand := flag.NewFlagSet("send", flag.ExitOnError)
	url := sendCommand.String("url", "http://localhost:8000", "url of the node.")
	nonce := sendCommand.Uint64("nonce", 0, "nonce for the transaction.")
	from := sendCommand.String("from", "", "who is sending the tx.")
	to := sendCommand.String("to", "", "who is receiving the tx.")
	value := sendCommand.Uint64("value", 0, "value to send.")
	tip := sendCommand.Uint64("tip", 0, "tip to send.")
	keysDir := sendCommand.String("dir", "block", "dir where private keys are stored.")
	account := sendCommand.String("account", "hamid", "who's private key to use to sign tx.")

	var data byteSliceFlag
	sendCommand.Var(&data, "data", "data to send")

	//custom usage
	sendCommand.Usage = func() {
		printSendUsage()
	}

	err := sendCommand.Parse(os.Args[2:])
	if err != nil {
		fmt.Println("Usage: admin send -account=<account> -dir=<keys_dir> -url=<node> -nonce=<nonce> -from=<from_id> -to=<to_id> -value=<value> -tip=<tip> [-data]")
		return fmt.Errorf("parse: %w", err)
	}

	return sendSignedTx(*account, *keysDir, *url, *nonce, *from, *to, *value, *tip, data)
}

func generateKey(dir string, accountName string) error {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		return fmt.Errorf("generateKey: %w", err)
	}

	if err := os.MkdirAll(dir, 0755); err != nil && !os.IsExist(err) {
		return fmt.Errorf("mkdirAll: %w", err)
	}

	filename := accountName + ".ecdsa"
	p := filepath.Join(dir, filename)

	if err := crypto.SaveECDSA(p, privateKey); err != nil {
		return fmt.Errorf("saveECDSA: %w", err)
	}

	return nil
}

func sendSignedTx(account, keys, url string, nonce uint64, from, to string, value, tip uint64, data []byte) error {
	//load private key
	key := fmt.Sprintf("%s/%s.ecdsa", keys, account)
	private, err := crypto.LoadECDSA(key)
	if err != nil {
		return fmt.Errorf("loadECDSA: %w", err)
	}

	fromID, err := database.NewAccountID(from)
	if err != nil {
		return err
	}

	toID, err := database.NewAccountID(to)
	if err != nil {
		return err
	}

	const chainID = 1
	tx, err := database.NewTX(chainID, nonce, fromID, toID, value, tip, data)
	if err != nil {
		return fmt.Errorf("newTX: %w", err)
	}

	signedTX, err := tx.Sign(private)
	if err != nil {
		return fmt.Errorf("sign: %w", err)
	}

	bs, err := json.Marshal(signedTX)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	//make POST Req
	resp, err := http.Post(fmt.Sprintf("%s/transactions/submit", url), "application/json", bytes.NewBuffer(bs))
	if err != nil {
		return fmt.Errorf("post req: %w", err)
	}

	defer resp.Body.Close()

	return nil
}
