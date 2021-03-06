package config

import "github.com/BurntSushi/toml"

type Global struct {
	DSPD        Dspd         `toml:"dspd"`
	Textd       Textd        `toml:"textd"`
	Bot         Bot          `toml:"bot"`
	Imaged      Imaged       `toml:"imaged"`
	BitmapFonts []BitmapFont `toml:"bitmapfont"`
}

type Dspd struct {
	GRPCServerAddress string `toml:"grpc_address"`
}

type Textd struct {
	GRPCServerAddress string `toml:"grpc_address"`
	FontPath          string `toml:"font_path"`
}

type BitmapFont struct {
	Name     string `toml:"name"`
	FileName string `toml:"filename"`
	High     int    `toml:"high"`
	Width    int    `toml:"width"`
}

type Imaged struct {
	GRPCServerAddress string `toml:"grpc_address"`
}

type Bot struct {
	SlackToken string `toml:"slack_token"`
}

func Load(path string) (*Global, error) {
	c := new(Global)
	_, err := toml.DecodeFile(path, &c)
	return c, err
}
