package parse_service

import (
	"encoding/hex"
	"fmt"
	"metaid-market-service/conf"
	"strings"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
)

const (
	PinProtocolIDTestnet string = "746573746964" //testid(HEX16)
	PinProtocolIDLivenet string = "6d6574616964" //metaid(HEX16)
)

type PersonalInformationNode struct {
	Operation     string `json:"operation"`
	Path          string `json:"path"`
	Encryption    string `json:"encryption"`
	Version       string `json:"cersion"`
	ContentType   string `json:"contentType"`
	ContentBody   []byte `json:"contentBody"`
	ContentLength uint64 `json:"contentLength"`
	ParentPath    string `json:"parentPath"`
	Protocols     bool   `json:"protocols"`
}

type Indexer struct {
	ChainParams *chaincfg.Params
	Block       interface{}
}

func PinProtocolID() string {
	if conf.Net == "testnet" {
		return PinProtocolIDTestnet
	}
	return PinProtocolIDLivenet
}

func getParentPath(path string) (parentPath string) {
	arr := strings.Split(path, "/")
	if len(arr) < 3 {
		return
	}
	parentPath = strings.Join(arr[0:len(arr)-1], "/")
	return
}

func (indexer *Indexer) ParsePins(witnessScript []byte) (pins []*PersonalInformationNode) {
	// Parse pins content from witness script
	tokenizer := txscript.MakeScriptTokenizer(0, witnessScript)
	for tokenizer.Next() {
		// Check inscription envelop header: OP_FALSE(0x00), OP_IF(0x63), PROTOCOL_ID
		if tokenizer.Opcode() == txscript.OP_FALSE {
			if !tokenizer.Next() || tokenizer.Opcode() != txscript.OP_IF {
				return
			}
			if !tokenizer.Next() || hex.EncodeToString(tokenizer.Data()) != PinProtocolID() {
				return
			}
			pinode := indexer.parseOnePin(&tokenizer)
			if pinode != nil {
				pins = append(pins, pinode)
			}
		}
	}
	return
}
func (indexer *Indexer) ParsePin(witnessScript []byte) (pinode *PersonalInformationNode) {
	// Parse pins content from witness script
	tokenizer := txscript.MakeScriptTokenizer(0, witnessScript)
	for tokenizer.Next() {
		// Check inscription envelop header: OP_FALSE(0x00), OP_IF(0x63), PROTOCOL_ID
		if tokenizer.Opcode() == txscript.OP_FALSE {
			if !tokenizer.Next() || tokenizer.Opcode() != txscript.OP_IF {
				return
			}
			if !tokenizer.Next() || hex.EncodeToString(tokenizer.Data()) != PinProtocolID() {
				fmt.Printf("PinMetaIDFlag-Hex: %s\n", hex.EncodeToString(tokenizer.Data()))
				fmt.Printf("PinMetaIDFlag: %s\n", string(tokenizer.Data()))
				return
			}
			pinode = indexer.parseOnePin(&tokenizer)
		}
	}
	return
}
func (indexer *Indexer) parseOnePin(tokenizer *txscript.ScriptTokenizer) *PersonalInformationNode {
	// Find any pushed data in the script. This includes OP_0, but not OP_1 - OP_16.
	var infoList [][]byte
	fmt.Printf("tokenizer.Data(): %s, %s\n", hex.EncodeToString(tokenizer.Data()), string(tokenizer.Data()))
	for tokenizer.Next() {
		if tokenizer.Opcode() == txscript.OP_ENDIF {
			break
		}
		infoList = append(infoList, tokenizer.Data())
		if len(tokenizer.Data()) > 520 {
			//log.Errorf("data is longer than 520")
			return nil
		}
	}
	// No OP_ENDIF
	if tokenizer.Opcode() != txscript.OP_ENDIF {
		return nil
	}
	// Error occurred
	if err := tokenizer.Err(); err != nil {
		return nil
	}
	if len(infoList) < 1 {
		return nil
	}
	for _, v := range infoList {
		fmt.Printf("infoList: %s, %s\n", hex.EncodeToString(v), string(v))
	}

	pinode := PersonalInformationNode{}
	pinode.Operation = strings.ToLower(string(infoList[0]))
	if len(infoList) < 6 {
		return nil
	}
	pinode.Path = strings.ToLower(string(infoList[1]))
	pinode.ParentPath = getParentPath(pinode.Path)
	encryption := "0"
	if infoList[2] != nil {
		encryption = string(infoList[2])
	}
	pinode.Encryption = encryption
	version := "0"
	if infoList[3] != nil {
		version = string(infoList[3])
	}
	pinode.Version = version
	contentType := "application/json"
	if infoList[4] != nil {
		contentType = strings.ToLower(string(infoList[4]))
	}
	pinode.ContentType = contentType
	var body []byte
	for i := 5; i < len(infoList); i++ {
		body = append(body, infoList[i]...)
	}
	pinode.ContentBody = body
	pinode.ContentLength = uint64(len(body))
	return &pinode
}
