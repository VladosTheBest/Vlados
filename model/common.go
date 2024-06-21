package model

type PagingMeta struct {
	Page   int                    `json:"page"`
	Count  int64                  `json:"count"`
	Limit  int                    `json:"limit"`
	Order  string                 `json:"order"`
	Filter map[string]interface{} `json:"filter"`
}

type GeneratedFile struct {
	Type     string `json:"filetype"`
	DataType string `json:"datatype"`
	Data     []byte `json:"data"`
}
