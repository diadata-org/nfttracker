package types

import "encoding/json"

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
