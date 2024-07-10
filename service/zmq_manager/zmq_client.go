package zmq_manager

import (
	"bytes"
	"fmt"
	"github.com/btcsuite/btcd/wire"
	zmq "github.com/pebbe/zmq4"
	"log"
	"metaid-market-service/conf"
)

func ZmqClientStart() {
	go zmqOneClientStart()
}

func zmqOneClientStart() {
	var (
		zmqHost string = conf.ZmqRawTxUrl
	)
	fmt.Printf("[ZMQ]zmqHost:%v\n", zmqHost)
	log.Println("zmq_manager connect to", zmqHost)
	q, _ := zmq.NewSocket(zmq.SUB)
	defer q.Close()
	//connection
	q.Connect(zmqHost)
	//sub topic
	q.SetSubscribe("rawtx")

	for {
		msg, _ := q.RecvMessage(0)

		//log.Printf("[ZMQ]new message\n")

		//handle

		var msgTx wire.MsgTx
		if err := msgTx.Deserialize(bytes.NewReader([]byte(msg[1]))); err != nil {
			continue
		}

	}
}
