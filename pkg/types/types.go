package types

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"math/big"
	"time"
)

type NFTCreation struct {
	Address string
	NFTType string
}

type NFTTransfer struct {
	Address       string
	From          string
	To            string
	Name          string
	Mint          bool
	TransactionID string
}

func (nc *NFTCreation) UnmarshalBinary(data []byte) error {
	if err := json.Unmarshal(data, &nc); err != nil {
		return err
	}
	return nil
}

func (nc NFTCreation) MarshalBinary() ([]byte, error) {
	return json.Marshal(nc)
}

type NFT struct {
	// NFTClass       NFTClass
	TokenID        string
	CreationTime   time.Time
	CreatorAddress string
	URI            string
	// @Attributes is a collection of attributes from on- and off-chain
	// TO DO: Should we split up into two fields?
	Attributes NFTAttributes
}

// NFTAttributes can be stored as jasonb in postgres:
// https://www.alexedwards.net/blog/using-postgresql-jsonb
type NFTAttributes map[string]interface{}

func (a NFTAttributes) Value() (driver.Value, error) {
	return json.Marshal(a)
}

func (a *NFTAttributes) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(b, &a)
}

// MarshalBinary for DefiProtocolState
func (n *NFT) MarshalBinary() ([]byte, error) {
	return json.Marshal(n)
}

// UnmarshalBinary for DefiProtocolState
func (n *NFT) UnmarshalBinary(data []byte) error {
	if err := json.Unmarshal(data, &n); err != nil {
		return err
	}
	return nil
}

type Asset struct {
	Symbol     string
	Name       string
	Address    string
	Decimals   uint8
	Blockchain string
}

type NFTTrade struct {
	NFT         NFT
	Price       *big.Int
	PriceUSD    float64
	FromAddress string
	ToAddress   string
	Currency    Asset
	BundleSale  bool
	BlockNumber uint64
	Timestamp   time.Time
	TxHash      string
	Exchange    string
}

// MarshalBinary for DefiProtocolState
func (ns *NFTTrade) MarshalBinary() ([]byte, error) {
	return json.Marshal(ns)
}

// UnmarshalBinary for DefiProtocolState
func (ns *NFTTrade) UnmarshalBinary(data []byte) error {
	if err := json.Unmarshal(data, &ns); err != nil {
		return err
	}
	return nil
}
