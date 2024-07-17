package pkg

import (
	"bytes"
	"encoding/json"
	"os"
)

type Config struct {
	Listen     string         `json:"listen"`
	Webroot    string         `json:"webroot"`
	Storage    *StorageConfig `json:"storages"`
	WistiaConf *WistiaConf    `json:"wistia"`
	DBConf     *DBConfig      `json:"db"`
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

	if len(this.TempDir) <= 0 {
		this.TempDir = os.TempDir()
	}

	if this.WistiaConf == nil || this.WistiaConf.WistiaApiKey == "" {
		conf := new(WistiaConf)
		conf.MarginWithENV()
		this.WistiaConf = conf
	}

	if this.DBConf == nil {
		this.DBConf = &DBConfig{
			FilePath: os.Getenv("DB_FILE_PATH"),
		}
	}

	if len(this.Listen) <= 0 {
		this.Listen = os.Getenv("LISTEN")
	}
	if len(this.Webroot) <= 0 {
		this.Webroot = os.Getenv("WEBROOT")
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
