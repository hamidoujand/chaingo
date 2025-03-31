package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/crypto"
)

func main() {
	if err := run(); err != nil {
		fmt.Println("admin <subcommand> [...args]")
		fmt.Println("==========================SubCommands===========================")
		fmt.Println("genkey: generate a ECDSA private key.")
		fmt.Println("=====================================================")
		fmt.Println(err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("subcommand needs to be passed")
	}

	switch os.Args[1] {
	case "genkey":
		genkeyCommand := flag.NewFlagSet("genkey", flag.ExitOnError)
		keyDir := genkeyCommand.String("dir", "", "directory to put the generated key.")
		accountName := genkeyCommand.String("account", "", "name of the account associate with the key.")

		genkeyCommand.Parse(os.Args[2:])

		if *keyDir == "" {
			fmt.Println("Usage: admin genkey -dir=<dir_path> -account=<accunt_name>")
			return fmt.Errorf("<dir> is a required argument")
		}

		if *accountName == "" {
			fmt.Println("Usage: admin genkey -dir=<dir_path> -account=<accunt_name>")
			return fmt.Errorf("<account> is a required argument")
		}

		return generateKey(*keyDir, *accountName)
	default:
		return fmt.Errorf("unknown command %q", os.Args[1])
	}

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
