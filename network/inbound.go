package network

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"github.com/harrisonhesslink/flatend"
	"log"
	"strconv"
	//"io/ioutil"
	"github.com/karai/go-karai/transaction"
	"github.com/karai/go-karai/util"
	// "time"
	//"encoding/json"
)

/*
This function handles request for transactions. It takes a top consensus tx hash. 100 txes per batch
*/
func (s *Server) HandleGetTxes(ctx *flatend.Context, request []byte) {
	command := BytesToCmd(request[:commandLength])

	var buff bytes.Buffer
	var payload GetTxes

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Println("ERROR HandleGetTxs: Failed to decode payload", err)
		return
	}
	log.Println(util.Rcv + " [" + command + "] Get Tx from: " + payload.Top_hash)
	last_hash := payload.Top_hash
	transactions := []transaction.Transaction{}

	if !s.Prtl.Dat.HaveTx(last_hash) {
		//nothing

	} else {
		log.Println(payload.FillData)
		if payload.FillData {

			db, connectErr := s.Prtl.Dat.Connect()
			defer db.Close()
			util.Handle("Error creating a DB connection: ", connectErr)

			for key, value := range payload.Contracts {
					//loop through to find oracle data
					row3, err := db.Queryx("SELECT * FROM "+s.Prtl.Dat.Cf.GetTableName()+" WHERE tx_type='2' AND tx_epoc=$1 ORDER BY tx_time DESC", key)
					if err != nil {
						panic(err)
					}
					defer row3.Close()
					for row3.Next() {
						var t2_tx transaction.Transaction
						err = row3.StructScan(&t2_tx)
						if err != nil {
							// handle this error
							log.Panic(err)
						}
						log.Println(t2_tx.Hash + " " + t2_tx.Type)

						if value == t2_tx.Hash {
							row3.Close()
							break
						}

						transactions = append(transactions, t2_tx)
					}
					err = row3.Err()
					if err != nil {
						log.Panic(err)
					}
				}
		} else {

			db, connectErr := s.Prtl.Dat.Connect()
			defer db.Close()
			util.Handle("Error creating a DB connection: ", connectErr)

			lhash := last_hash
			//Grab all first txes on epoc 
			rows, err := db.Queryx("SELECT * FROM " + s.Prtl.Dat.Cf.GetTableName() + " WHERE tx_type='1' ORDER BY tx_time DESC")
			if err != nil {
				panic(err)
			}
			defer rows.Close()
			for rows.Next() {
				var this_tx transaction.Transaction
				err = rows.StructScan(&this_tx)
				if err != nil {
					// handle this error
					log.Panic(err)
				}

				log.Println(this_tx.Hash + " " + this_tx.Type)
				transactions = append(transactions, this_tx)

				//loop through to find contracts
				row2, err := db.Queryx("SELECT * FROM "+s.Prtl.Dat.Cf.GetTableName()+" WHERE tx_type='3' AND tx_prnt=$1 ORDER BY tx_time DESC", this_tx.Hash)
				if err != nil {
					panic(err)
				}
				defer row2. Close()
				for row2.Next() {
					var t_tx transaction.Transaction
					err = row2.StructScan(&t_tx)
					if err != nil {
						// handle this error
						log.Panic(err)
					}
					log.Println(this_tx.Hash + " " + t_tx.Type)
					transactions = append(transactions, t_tx)
					//loop through to find oracle data
					row3, err := db.Queryx("SELECT * FROM "+s.Prtl.Dat.Cf.GetTableName()+" WHERE tx_type='2' AND tx_epoc=$1 ORDER BY tx_time DESC", t_tx.Epoc)
					if err != nil {
						panic(err)
					}
					defer row3.Close()
					for row3.Next() {
						var t2_tx transaction.Transaction
						err = row3.StructScan(&t2_tx)
						if err != nil {
							// handle this error
							log.Panic(err)
						}
						log.Println(this_tx.Hash + " " + t2_tx.Type)
						transactions = append(transactions, t2_tx)
					}
					err = row3.Err()
					if err != nil {
						log.Panic(err)
					}
				}
				err = rows.Err()
				if err != nil {
					log.Panic(err)
				}

				if lhash == this_tx.Hash {
					rows.Close()
					break;
				}
			}

			// get any error encountered during iteration
			err = rows.Err()
			if err != nil {
				log.Panic(err)
			}
		}

		var txes [][]byte
		log.Println("Sending txes: " + strconv.Itoa(len(transactions)))
		for i := len(transactions)-1; i >= 0; i-- {
			
			txes = append(txes, transactions[i].Serialize())
			if (i % 100) == 0 {
				log.Println("Sending: " + strconv.Itoa(len(txes)))
				data := GOB_BATCH_TX{txes, len(transactions)}
				payload := GobEncode(data)
				request := append(CmdToBytes("batchtx"), payload...)
		
				go s.SendData(ctx, request)
				txes =  [][]byte{}
			}
		}
	}
}

func (s *Server) HandleGetData(ctx *flatend.Context, request []byte) {
	command := BytesToCmd(request[:commandLength])

	var buff bytes.Buffer
	var payload GetData

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}

	if payload.Type == "tx" {

		//tx := s.Prtl.Dat.GetTransaction(payload.ID)

		//s.SendTx(s.GetProviderFromID(&ctx.ID), tx)
	}
	log.Println(util.Rcv + " [" + command + "] Data Request from: " + ctx.ID.Pub.String())
}

func (s *Server) HandleBatchTx(ctx *flatend.Context, request []byte) {
	if s.sync == true {
		command := BytesToCmd(request[:commandLength])

		var buff bytes.Buffer
		var payload GOB_BATCH_TX

		buff.Write(request[commandLength:])
		dec := gob.NewDecoder(&buff)
		err := dec.Decode(&payload)
		if err != nil {
			log.Panic(err)
		}

		db, connectErr := s.Prtl.Dat.Connect()
		defer db.Close()
		util.Handle("Error creating a DB connection: ", connectErr)

		var txPrev string
		_ = db.QueryRow("SELECT tx_hash FROM " + s.Prtl.Dat.Cf.GetTableName() + " WHERE tx_type='1' ORDER BY tx_time DESC").Scan(&txPrev)

		var need_fill bool
		for _, tx_ := range payload.Batch {

			tx := transaction.DeserializeTransaction(tx_)

			if tx.Hash == txPrev {
				need_fill = true
			}

			if s.Prtl.Dat.HaveTx(tx.Prev) {
				if !s.Prtl.Dat.HaveTx(tx.Hash) {
					s.Prtl.Dat.CommitDBTx(tx)
				}
			}
		}

		if need_fill {
			go s.SendGetTxes(ctx, true, s.GetContractMap())
		}

		percentage_float := float64(payload.TotalSent) / float64(s.tx_need) * 100
		percentage_string := fmt.Sprintf("%.2f", percentage_float)
		log.Println(util.Rcv + " [" + command + "] Received Transactions. Sync %:" + percentage_string + "[" + strconv.Itoa(payload.TotalSent) + "/" + strconv.Itoa(s.tx_need) + "]")
		if payload.TotalSent == s.tx_need {
			s.tx_need = 0
			s.sync = false
		}
	}
}

func (s *Server) GetContractMap() map[string]string {

	db, connectErr := s.Prtl.Dat.Connect()
	defer db.Close()
	util.Handle("Error creating a DB connection: ", connectErr)

	var Contracts map[string]string
	//loop through to find oracle data
	row3, err := db.Queryx("SELECT * FROM "+s.Prtl.Dat.Cf.GetTableName()+" WHERE tx_type='3' ORDER BY tx_time DESC")
	if err != nil {
		panic(err)
	}
	defer row3.Close()
	for row3.Next() {
		var this_tx transaction.Transaction
		err = row3.StructScan(&this_tx)
		if err != nil {
			// handle this error
			log.Panic(err)
		}
		var data_prev string
		_ = db.QueryRow("SELECT tx_hash FROM " + s.Prtl.Dat.Cf.GetTableName() + " WHERE tx_type='1' ORDER BY tx_time DESC").Scan(&data_prev)
		Contracts[this_tx.Hash] = data_prev
	}
	err = row3.Err()
	if err != nil {
		log.Panic(err)
	}

	return Contracts
}


func (s *Server) HandleTx(ctx *flatend.Context, request []byte) {
	command := BytesToCmd(request[:commandLength])

	var buff bytes.Buffer
	var payload GOB_TX

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}
	txData := payload.TX
	tx := transaction.DeserializeTransaction(txData)

	log.Println(util.Rcv + " [" + command + "] Transaction: " + tx.Hash)

	if s.Prtl.Dat.HaveTx(tx.Prev) {
		if !s.Prtl.Dat.HaveTx(tx.Hash) {
			s.Prtl.Dat.CommitDBTx(tx)
		}
	}
}

func (s *Server) HandleVersion(ctx *flatend.Context, request []byte) {
	command := BytesToCmd(request[:commandLength])
	var buff bytes.Buffer
	var payload Version

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}

	if payload.TxSize > s.Prtl.Dat.GetDAGSize() {
		//lock in the first node
		if s.sync == false {
			go s.SendGetTxes(ctx, false, map[string]string{})
			s.sync = true
			s.tx_need = payload.TxSize - s.Prtl.Dat.GetDAGSize()
		}
	}

	log.Println(util.Rcv + " [" + command + "] Node has Num Tx: " + strconv.Itoa(payload.TxSize))
}

func (s *Server) HandleConnection(req []byte, ctx *flatend.Context) {

	command := BytesToCmd(req[:commandLength])
	switch command {
	case "gettxes":
		go s.HandleGetTxes(ctx, req)
	case "getdata":
		go s.HandleGetData(ctx, req)
	case "tx":
		go s.HandleTx(ctx, req)
	case "batchtx":
		go s.HandleBatchTx(ctx, req)
	case "version":
		go s.HandleVersion(ctx, req)
	}

}

