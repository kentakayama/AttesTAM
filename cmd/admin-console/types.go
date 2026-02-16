package main

type Agent struct {
	KID        string     `json:"kid"`
	Attributes Attribute  `json:"attribute"`
	WappList   []WappItem `json:"wapp_list"`
}

type Attribute struct {
	Ueid string `json:"ueid"`
}

type WappItem struct {
	Name string `json:"name"`
	Ver  int    `json:"ver"`
}

type Manifest struct {
	Name string `json:"name"`
	Ver  int    `json:"version"`
}
