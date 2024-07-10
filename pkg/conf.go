package pkg

import (
	"bytes"
	"encoding/json"
	"os"
)

type Config struct {
	ParserType string         `json:"parser"` // sendgrid | pop3
	Storage    *StorageConfig `json:"storages"`
	TempDir    string
}

func NewConfigFromLocal(filename string) (*Config, error) {
	conf := &Config{}
	err := conf.load(filename)
	if err == nil && len(conf.TempDir) <= 0 {
		conf.TempDir = os.TempDir()
	}
	return conf, err
}

func (this *Config) MarginWithENV() {
	if this.Storage == nil || this.Storage.S3 == nil {
		this.Storage = &StorageConfig{
			S3: LoadS3ConfigWithEnv(),
		}
	}
}

func (c *Config) load(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	err = decoder.Decode(c)
	if err != nil {
		Log.Error(err)
	}
	return err
}

func (c *Config) ToJSON() (string, error) {
	jsonBin, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	var str bytes.Buffer
	_ = json.Indent(&str, jsonBin, "", "  ")
	return str.String(), nil
}

func (c *Config) Save(saveAs string) error {
	file, err := os.Create(saveAs)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer file.Close()
	data, err := json.MarshalIndent(c, "", "    ")
	if err != nil {
		Log.Error(err)
		return err
	}
	_, err = file.Write(data)
	if err != nil {
		Log.Error(err)
	}
	return err
}
