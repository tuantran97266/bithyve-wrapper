package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/bithyve/bithyve-wrapper/electrs"
	"github.com/bithyve/bithyve-wrapper/format"

	erpc "github.com/Varunram/essentials/rpc"
)

func wait() {
	time.Sleep(100 * time.Millisecond)
}

func checkReq(w http.ResponseWriter, r *http.Request) ([]string, error) {
	var arr []string
	err := erpc.CheckPost(w, r) // check origin of request as well if needed
	if err != nil {
		erpc.ResponseHandler(w, erpc.StatusNotFound)
		log.Println(err)
		return arr, err
	}
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		erpc.ResponseHandler(w, erpc.StatusBadRequest)
		log.Println(err)
		return arr, err
	}
	var rf format.RequestFormat
	err = json.Unmarshal(data, &rf)
	if err != nil {
		erpc.ResponseHandler(w, erpc.StatusInternalServerError)
		log.Println(err)
		return arr, err
	}

	arr = rf.Addresses

	// filter through list to remove duplicates
	nodupsMap := make(map[string]bool)
	var nodups []string

	for _, elem := range arr {
		if _, value := nodupsMap[elem]; !value {
			nodupsMap[elem] = true
			nodups = append(nodups, elem)
		}
	}

	return nodups, nil
}

func multiAddr(w http.ResponseWriter, r *http.Request,
	arr []string) ([]format.MultigetAddrReturn, error) {

	x := make([]format.MultigetAddrReturn, len(arr))
	currentBh, err := electrs.CurrentBlockHeight()
	if err != nil {
		erpc.ResponseHandler(w, erpc.StatusInternalServerError)
		log.Println(err)
		return x, err
	}

	for i, elem := range arr {
		x[i].Address = elem // store the address of the passed elements
		// send the request out
		go func(i int, elem string) {
			allTxs, err := electrs.GetTxsAddress(elem)
			if err == nil {
				x[i].TotalTransactions = float64(len(allTxs))
				x[i].Transactions = allTxs
				x[i].ConfirmedTransactions, x[i].UnconfirmedTransactions = 0, 0
				for j := range x[i].Transactions {
					if x[i].Transactions[j].Status.Confirmed {
						x[i].Transactions[j].NumberofConfirmations =
							currentBh - x[i].Transactions[j].Status.BlockHeight
					} else {
						x[i].Transactions[j].NumberofConfirmations = 0
					}
				}
				go func(i int, elem string) {
					x[i].ConfirmedTransactions, x[i].UnconfirmedTransactions =
						electrs.GetBalanceCount(elem)
				}(i, elem)
			} else {
				log.Println("error in gettxsaddress call: ", err)
			}
		}(i, elem)
	}

	wait()
	return x, nil
}

func multiBalance(arr []string, w http.ResponseWriter, r *http.Request) format.BalanceReturn {
	var x format.BalanceReturn
	for _, elem := range arr {
		// send the request out
		tBalance, tUnconfirmedBalance := 0.0, 0.0
		go func(elem string) {
			log.Println("calling the balances endpoint")
			tBalance, tUnconfirmedBalance = electrs.GetBalanceAddress(elem)
			x.Balance += tBalance
			x.UnconfirmedBalance += tUnconfirmedBalance
		}(elem)
	}

	wait()
	return x
}

// MultiUtxos gets the utxos associated with multiple addresses
func MultiUtxos() {
	// make a curl request out to lcoalhost and get the ping response
	http.HandleFunc("/utxos", func(w http.ResponseWriter, r *http.Request) {
		// validate if the person requesting this is a vlaid user on the platform
		arr, err := checkReq(w, r)
		if err != nil {
			return
		}

		var result [][]format.Utxo
		for _, elem := range arr {
			// send the request out
			go func(elem string) {
				tempTxs, err := electrs.GetUtxosAddress(elem)
				if err != nil {
					erpc.ResponseHandler(w, http.StatusInternalServerError)
					log.Println(err)
					return
				}
				result = append(result, tempTxs)
			}(elem)
		}
		wait()
		erpc.MarshalSend(w, result)
	})
}

// MultiData gets all data associated with a particular address
func MultiData() {
	// make a curl request out to localhost and get the ping response
	http.HandleFunc("/data", func(w http.ResponseWriter, r *http.Request) {
		// validate if the person requesting this is a vlaid user on the platform
		arr, err := checkReq(w, r)
		if err != nil {
			return
		}

		x, err := multiAddr(w, r, arr)
		if err != nil {
			return
		}

		erpc.MarshalSend(w, x)
	})
}

// MultiBalTxs combines the balance and Multigetaddr endpoints
func MultiBalTxs() {
	// make a curl request out to lcoalhost and get the ping response
	http.HandleFunc("/baltxs", func(w http.ResponseWriter, r *http.Request) {
		arr, err := checkReq(w, r)
		if err != nil {
			return
		}

		var ret format.BalTxReturn
		ret.Balance = multiBalance(arr, w, r)
		ret.Transactions, err = multiAddr(w, r, arr)
		if err != nil {
			return
		}

		erpc.MarshalSend(w, ret)
	})
}

// MultiBalances gets the net balance associated with multiple addresses
func MultiBalances() {
	// make a curl request out to lcoalhost and get the ping response
	http.HandleFunc("/balances", func(w http.ResponseWriter, r *http.Request) {
		// validate if the person requesting this is a vlaid user on the platform
		arr, err := checkReq(w, r)
		if err != nil {
			return
		}

		x := multiBalance(arr, w, r)
		erpc.MarshalSend(w, x)
	})
}

// MultiTxs gets the transactions associated with mutliple addresses
func MultiTxs() {
	// make a curl request out to lcoalhost and get the ping response
	http.HandleFunc("/txs", func(w http.ResponseWriter, r *http.Request) {
		// validate if the person requesting this is a vlaid user on the platform
		arr, err := checkReq(w, r)
		if err != nil {
			return
		}

		var x format.TxReturn
		for _, elem := range arr {
			// send the request out
			go func(elem string) {
				tempTxs, err := electrs.GetTxsAddress(elem)
				if err != nil {
					erpc.ResponseHandler(w, http.StatusInternalServerError)
					log.Println(err)
					return
				}
				x.Txs = append(x.Txs, tempTxs)
			}(elem)
		}

		wait()
		erpc.MarshalSend(w, x)
	})
}

// GetFees gets the current fee estimate from esplora
func GetFees() {
	http.HandleFunc("/fees", func(w http.ResponseWriter, r *http.Request) {
		// validate if the person requesting this is a vlaid user on the platform
		err := erpc.CheckPost(w, r) // check origin of request as well if needed
		if err != nil {
			erpc.ResponseHandler(w, erpc.StatusNotFound)
			log.Println(err)
			return
		}

		var x format.FeeResponse
		body := electrs.ElectrsURL + "/fee-estimates"
		erpc.GetAndSendJson(w, body, x)
	})
}

// PostTx posts a transaction to the blockchain
func PostTx() {
	http.HandleFunc("/tx", func(w http.ResponseWriter, r *http.Request) {
		// validate if the person requesting this is a valid user on the platform
		err := erpc.CheckPost(w, r) // check origin of request as well if needed
		if err != nil {
			erpc.ResponseHandler(w, erpc.StatusNotFound)
			log.Println(err)
			return
		}
		body := electrs.ElectrsURL + "/tx"
		data, err := erpc.PostRequest(body, r.Body)
		if err != nil {
			erpc.ResponseHandler(w, erpc.StatusInternalServerError)
			log.Println("could not submit transacation to testnet: ", err)
			return
		}
		var x interface{}
		err = json.Unmarshal(data, &x)
		if err != nil {
			erpc.ResponseHandler(w, erpc.StatusInternalServerError)
			log.Println("error while unmarshalling json struct", string(data))
			return
		}
		erpc.MarshalSend(w, x)
	})
}
