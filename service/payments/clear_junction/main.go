package clear_junction

import (
	"bytes"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

type ClearJunctionProcessor struct {
	xApiKey           string
	apiPasswordHashed string
	apiUrl            string
	walletUUID        string
}

func Init(xApiKey, apiPassword, apiUrl, walletUUID string) *ClearJunctionProcessor {

	apiPasswordHash := newHash(apiPassword)

	return &ClearJunctionProcessor{
		xApiKey:           strings.ToUpper(xApiKey),
		apiPasswordHashed: strings.ToUpper(apiPasswordHash),
		apiUrl:            apiUrl,
		walletUUID:        walletUUID,
	}
}

func newHash(input string) string {
	h := sha512.New()
	_, _ = h.Write([]byte(input))
	return hex.EncodeToString(h.Sum(nil))
}

func (p *ClearJunctionProcessor) sign(t time.Time, body []byte) string {
	hash := ""

	hash += p.xApiKey
	hash += t.Format(time.RFC3339)
	hash += p.apiPasswordHashed

	hash += strings.ToUpper(string(body))

	// key + date + pass + body
	return newHash(hash)
}

func (p *ClearJunctionProcessor) request(method, url string, body []byte) ([]byte, int, error) {

	client := &http.Client{}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	t := time.Now()

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Date", t.Format(time.RFC3339))
	req.Header.Set("X-API-KEY", p.xApiKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.sign(t, body)))

	resp, err := client.Do(req)

	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	defer resp.Body.Close()
	respBody, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	return respBody, resp.StatusCode, nil
}

func (p *ClearJunctionProcessor) CheckWallet() ([]byte, error) {

	url := fmt.Sprintf("%s/v7/bank/wallets/%s", p.apiUrl, p.walletUUID)

	respBody, _, err := p.request(http.MethodGet, url, nil)

	return respBody, err
}

func (p *ClearJunctionProcessor) CheckIBAN(iban string) (*CheckIbanResponse, error) {

	url := fmt.Sprintf("%s/v7/gate/checkRequisite/bankTransfer/eu/iban/%s", p.apiUrl, iban)

	respBytes, _, err := p.request(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	result := CheckIbanResponse{}

	if err := json.Unmarshal(respBytes, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (p *ClearJunctionProcessor) ActionWithTransactions(transactions []string, action TransactionAction) (*CheckIbanResponse, error) {

	url := fmt.Sprintf("%s/v7/gate/transactionAction/%s", p.apiUrl, action)

	ac := actionWithTransaction{OrderReferenceArray: transactions}

	body, err := json.Marshal(ac)
	if err != nil {
		return nil, err
	}

	respBody, _, err := p.request(http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}

	result := CheckIbanResponse{}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}

	return &result, nil
}
