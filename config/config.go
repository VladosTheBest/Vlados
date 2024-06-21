package config

import (
	"fmt"

	"github.com/ericlagergren/decimal"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/featureflags"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/monitor"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/net/kafka"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/net/redis"
)

// Config structure
type Config struct {
	Server          ServerConfig
	Engine          EngineConfig          `mapstructure:"engine"`
	Kafka           kafka.Config          `mapstructure:"kafka"`
	DatabaseCluster DatabaseClusterConfig `mapstructure:"database_cluster"`
	Redis           redis.Config
	Preprocessors   Preprocessors
	Distribution    Distribution         `mapstructure:"distribution"`
	Crons           Crons                `mapstructure:"crons"`
	Unleash         featureflags.Config  `mapstructure:"unleash"`
	CoinValuesAPI   CoinValuesApiConfig  `mapstructure:"coin_values_api"`
	BonusAccount    BonusAccountConfig   `mapstructure:"bonus_account"`
	Staking         StakingConfig        `mapstructure:"staking"`
	Emails          []string             `mapstructure:"emails"`
	Fee             FeeConfig            `mapstructure:"fee"`
	AdvCash         AdvCashConfig        `mapstructure:"adv_cash"`
	Bots            BotsConfig           `mapstructure:"bots"`
	ClearJunction   ClearJunctionConfig  `mapstructure:"clear_junction"`
	FirebaseClient  FirebaseClientConfig `mapstructure:"firebase_client"`
	ReferralConfig  ReferralsConfig      `mapstructure:"referral_config"`
	LeadBonusConfig LeadBonusConfig      `mapstructure:"lead_bonus"`
	OtcDeskConfig   OtcDeskConfig        `mapstructure:"otc_desk"`
	AWS             AWS                  `mapstructure:"aws"`
	Datasync        DatasyncConfig       `mapstructure:"datasync"`
}

type DatasyncConfig struct {
	Url string `mapstructure:"url"`
}

type EngineConfig struct {
	SeqIDOffset int64 `mapstructure:"seq_id_offset"`
	IDsOffset   int64 `mapstructure:"ids_offset"`
}

type ReferralsConfig struct {
	L1 float64 `mapstructure:"L1"`
	L2 float64 `mapstructure:"L2"`
	L3 float64 `mapstructure:"L3"`
}

type ClearJunctionConfig struct {
	ApiUrl      string            `mapstructure:"apiUrl"`
	ApiKey      string            `mapstructure:"apiKey"`
	ApiPassword string            `mapstructure:"apiPassword"`
	WalletUUID  string            `mapstructure:"walletUUID"`
	PostbackUrl string            `mapstructure:"postbackUrl"`
	Assets      map[string]string `mapstructure:"assets"`
	Requisites  struct {
		BankName          string `mapstructure:"bank_name"`
		Address           string `mapstructure:"address"`
		Swift             string `mapstructure:"swift"`
		Iban              string `mapstructure:"iban"`
		SortCode          string `mapstructure:"sort_code"`
		AccountNumber     string `mapstructure:"account_number"`
		AccountHolderName string `mapstructure:"account_holder_name"`
	} `mapstructure:"requisites"`
}

type BotsConfig struct {
	Grid   BotGridConfig  `mapstructure:"grid" json:"grid"`
	Trend  BotTrendConfig `mapstructure:"trend" json:"trend"`
	Limits map[string]struct {
		Max float64 `mapstructure:"max" json:"max"`
		Min float64 `mapstructure:"min" json:"min"`
	} `mapstructure:"limits"`
}

type BotGridConfig struct {
	ApiEndpoints struct {
		Start string `mapstructure:"start"`
		Stop  string `mapstructure:"stop"`
	} `mapstructure:"api_endpoints" json:"-"`
	Limits map[string]struct {
		Max float64 `mapstructure:"max" json:"max"`
		Min float64 `mapstructure:"min" json:"min"`
	} `mapstructure:"limits" json:"limits"`
}

type BotTrendConfig struct {
	ApiEndpoints struct {
		Start string `mapstructure:"start"`
		Stop  string `mapstructure:"stop"`
	} `mapstructure:"api_endpoints" json:"-"`
	Limits map[string]struct {
		Max float64 `mapstructure:"max" json:"max"`
		Min float64 `mapstructure:"min" json:"min"`
	} `mapstructure:"limits" json:"limits"`
}

type FeeConfig struct {
	General struct {
		Taker float64 `mapstructure:"taker"`
		Maker float64 `mapstructure:"maker"`
	} `mapstructure:"general"`
	BonusAccount struct {
		Taker float64 `mapstructure:"taker"`
		Maker float64 `mapstructure:"maker"`
	} `mapstructure:"bonus_account"`
}

type FirebaseClientConfig struct {
	ApiKey string `mapstructure:"api_key" json:"grid"`
}

func (cfg *BonusAccountConfig) GetRiskLevel() *decimal.Big {
	return conv.NewDecimalWithPrecision().SetFloat64(cfg.RiskLevel)
}

func (cfg *BonusAccountConfig) GetPeriodsMap() map[int64]*BonusAccountPeriod {
	if cfg.bonusAccountPeriodsMap == nil {
		list := map[int64]*BonusAccountPeriod{}
		for _, bonusAccountPeriod := range cfg.Periods {
			list[bonusAccountPeriod.Period] = bonusAccountPeriod
		}
		cfg.bonusAccountPeriodsMap = list
	}
	return cfg.bonusAccountPeriodsMap
}

type BonusAccountConfig struct {
	AmountForContractStep float64 `mapstructure:"amount_for_contract_step"`
	MaxContractsPerUser   uint64  `mapstructure:"max_contracts_per_user"`
	RiskLevel             float64 `mapstructure:"risk_level"`
	Limits                map[string]struct {
		Max float64 `mapstructure:"max" json:"max"`
		Min float64 `mapstructure:"min" json:"min"`
	} `mapstructure:"limits"`
	Periods                []*BonusAccountPeriod `mapstructure:"periods"`
	bonusAccountPeriodsMap map[int64]*BonusAccountPeriod
	Landing                struct {
		Multipliers map[string]struct {
			Contracts  float64 `mapstructure:"contracts"`
			Invested   float64 `mapstructure:"invested"`
			BonusPayed float64 `mapstructure:"bonus_payed"`
		} `mapstructure:"multipliers"`
	} `mapstructure:"landing"`
}

type BonusAccountPeriod struct {
	Period               int64   `mapstructure:"period" json:"period"`
	Percent              float64 `mapstructure:"percent" json:"percent"`
	VolumeToTradePercent float64 `mapstructure:"volume_to_trade" json:"volume_to_trade"`
}

type StakingConfig struct {
	Limits map[string]struct {
		Max float64 `mapstructure:"max" json:"max"`
		Min float64 `mapstructure:"min" json:"min"`
	} `mapstructure:"limits"`
	Periods    []*StakingPeriodConfig `mapstructure:"periods"`
	periodsMap map[string]*StakingPeriodConfig
}

type StakingPeriodConfig struct {
	Period         string             `mapstructure:"period" json:"period"`
	Percent        map[string]float64 `mapstructure:"percent" json:"percent"`
	PayoutInterval string             `mapstructure:"payout_interval" json:"payout_interval"`
}

func (cfg *StakingConfig) GetPeriodsMap() map[string]*StakingPeriodConfig {
	if cfg.periodsMap == nil {
		list := map[string]*StakingPeriodConfig{}
		for _, bonusAccountPeriod := range cfg.Periods {
			list[bonusAccountPeriod.Period] = bonusAccountPeriod
		}
		cfg.periodsMap = list
	}
	return cfg.periodsMap
}

// ServerConfig structure
type ServerConfig struct {
	Monitoring                          monitor.Config           `mapstructure:"monitoring"`
	API                                 APIConfig                `mapstructure:"api"`
	Admin                               AdminConfig              `mapstructure:"admin"`
	Verification                        KYCVerificationConfig    `mapstructure:"verification"`
	Sendgrid                            SendgridConfig           `mapstructure:"sendgrid"`
	KYC                                 KYCConfig                `mapstructure:"kyc"`
	Twillio                             TwillioConfig            `mapstructure:"twillio"`
	Candles                             CandlesServiceConfig     `mapstructure:"candles"`
	Socket                              Socket                   `mapstructure:"socket"`
	GeeTest                             GeeTest                  `mapstructure:"geetest"`
	GeeTestV4                           GeeTest                  `mapstructure:"geetest_v4"`
	Info                                InfoConfig               `mapstructure:"info"`
	Debug                               DebugConfig              `mapstructure:"debug"`
	ManualTransactions                  ManualTransactionsConfig `mapstructure:"manual_transactions"`
	MMAccounts                          []string                 `mapstructure:"mm_accounts"`
	BurnedTokenApi                      BurnedTokenApiConfig     `mapstructure:"burned_tokens_api"`
	ManualDistributionRecoveryAccountId uint64                   `mapstructure:"manual_distribution_recovery_account_id"`
	PrdxDistributorUser                 string                   `mapstructure:"prdx_distributor_user"`
	PrdxUnusedFoundsUser                string                   `mapstructure:"prdx_unused_founds_user"`
	KYB                                 KYBConfig                `mapstructure:"kyb"`
	Card                                CardConfig               `maostructure:"card"`
	AWS                                 AWS                      `mapstructure:"aws"`

	StatsEndpoint                string `mapstructure:"stats_endpoint"`
	TradesEndpoint               string `mapstructure:"trades_endpoint"`
	QuoteVolumeDayBeforeEndpoint string `mapstructure:"quote_volume_day_before_endpoint"`
}

// Distribution godoc
type Distribution struct {
	Coin         string `mapstructure:"coin"`
	BotID        uint64 `mapstructure:"bot_id"`
	RevenueShare string `mapstructure:"revenue_share"`
}

// Crons - mapping of ids to execution frequency
type Crons map[string]string

type Socket struct {
	Workers             uint
	Commands            Commands
	CallRestartServices bool `mapstructure:"call_restart_services"`
	Endpoints           struct {
		WebsocketService          string `mapstructure:"websocket_service"`
		WebsocketGeneratorService string `mapstructure:"websocket_generator_service"`
	} `mapstructure:"endpoints"`
}

type Commands struct {
	InitialPublicCoinValues  string `mapstructure:"initial_public_coin_values"`
	InitialPublicMarket      string `mapstructure:"initial_public_market"`
	InitialPublicTrades      string `mapstructure:"initial_public_trades"`
	InitialPublicDepth       string `mapstructure:"initial_public_depth"`
	InitialUserTrades        string `mapstructure:"initial_user_trades"`
	InitialUserOrders        string `mapstructure:"initial_user_orders"`
	InitialUserBalances      string `mapstructure:"initial_user_balances"`
	InitialUserNotifications string `mapstructure:"initial_user_notifications"`
}

// KYCConfig structure
type KYCConfig struct {
	MerchantID       string `mapstructure:"merchant_id"`
	MerchantPassword string `mapstructure:"merchant_password"`
	MerchantSchema   string `mapstructure:"merchant_schema"`
	MerchantHost     string `mapstructure:"merchant_host"`
	MerchantPrefix   string `mapstructure:"merchant_prefix"`
	CallbackUrl      string `mapstructure:"callback_url"`
}

func (c KYCConfig) GetMerchantUrl() string {
	return fmt.Sprintf("%s://%s%s", c.MerchantSchema, c.MerchantHost, c.MerchantPrefix)
}

func (c KYCConfig) BuildAPIUrl(relativePath string) string {
	return c.GetMerchantUrl() + relativePath
}

type DebugConfig struct {
	AllowedIPs            string `mapstructure:"allowed_ips"`
	MaxNumberOfGoroutines int    `mapstructure:"max_number_of_goroutines"`
}
type ManualTransactionsConfig struct {
	ConfirmingUsers []string `mapstructure:"confirming_users"`
	ConfirmUrl      string   `mapstructure:"confirm_url"`
}

type BurnedTokenApiConfig struct {
	Url string `mapstructure:"url"`
}

// Card structure
type CardConfig struct {
	UserName      string `mapstructure:"user_name"`
	Password      string `mapstructure:"password"`
	Key           string `mapstructure:"key"`
	AgreementCode string `mapstructure:"agreement_code"`
}

// TwillioConfig structure
type TwillioConfig struct {
	URL             string `mapstructure:"url"`
	VerificationUrl string `mapstructure:"verification_url"`
	APIKey          string `mapstructure:"api_key"`
	AccountSID      string `mapstructure:"account_sid"`
	AuthToken       string `mapstructure:"auth_token"`
	ServiceSID      string `mapstructure:"service_sid"`
}

type CoinValuesApiConfig struct {
	UrlCoinValues string `mapstructure:"url_coin_values"`
	UrlLastPrices string `mapstructure:"url_last_prices"`
}

// CandlesServiceConfig structure
type CandlesServiceConfig struct {
	URL string `mapstructure:"url"`
}

// Preprocessors structure
type Preprocessors struct {
	QueueManager QueueManager `mapstructure:"queue_manager"`
}

// QueueManager config
type QueueManager struct {
	Type     string
	Patterns map[string]string
	Inputs   []string
	Outputs  []string
}

// SendgridConfig structure
type SendgridConfig struct {
	Key       string
	From      string
	Templates map[string]map[string]string
}

// APIConfig structure
type APIConfig struct {
	Port              int
	KeepAlive         bool `mapstructure:"keep_alive"`
	Domain            string
	JWTTokenSecret    string `mapstructure:"jwt_token_secret"`
	JWA2FATokenSecret string `mapstructure:"jwt_require_2fa_token"`
}

type InfoConfig struct {
	Contact struct {
		Email string `mapstructure:"email"`
	} `mapstructure:"contact"`
}

// AdminConfig structure
type AdminConfig struct {
	Domain string
}

// KYCVerificationConfig structure
type KYCVerificationConfig struct {
	Email string
}

// DatabaseClusterConfig structure
type DatabaseClusterConfig struct {
	Writer      DatabaseConfig `mapstructure:"writer"`
	Reader      DatabaseConfig `mapstructure:"reader"`
	ReaderAdmin DatabaseConfig `mapstructure:"reader_admin"`
}

// DatabaseConfig structure
type DatabaseConfig struct {
	Type            string // postgres / mysql
	Host            string
	Username        string
	Password        string
	Name            string
	SSLmode         string `mapstructure:"sslmode"`
	ApplicationName string `mapstructure:"application_name"`
	Port            int
}

// GeeTest structure
type GeeTest struct {
	ID  string `mapstructure:"id"`
	Key string `mapstructure:"key"`
}

// LoadConfig Load server configuration from the yaml file
func LoadConfig(viperConf *viper.Viper) Config {
	var config Config

	err := viperConf.Unmarshal(&config)
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to decode config into struct")
	}
	config.Preprocessors.QueueManager.Patterns["sync_data"] = "sync_data"
	config.Preprocessors.QueueManager.Outputs = append(config.Preprocessors.QueueManager.Outputs, "sync_data")
	return config
}

// OpenConfig godoc
func OpenConfig(file string) {
	// Don't forget to read config either from cfgFile, from current directory or from home directory!
	if file != "" {
		// Use config file from the flag.
		viper.SetConfigFile(file)
	}

	viper.SetConfigType("yaml")
	viper.SetConfigName(".config")
	viper.AddConfigPath(".")                     // First try to load the config from the current directory
	viper.AddConfigPath("$HOME")                 // Then try to load it from the HOME directory
	viper.AddConfigPath("/etc/exchange_api_v2/") // As a last resort try to load it from /etc/
	viper.SetEnvPrefix("CFG")
	viper.AutomaticEnv()
	setDefaultVariables()

	err := viper.ReadInConfig() // Find and read the config file
	if err != nil {             // Handle errors reading the config file
		log.Fatal().Err(err).Msg("Unable to read configuration file")
	}
}

type AdvCashConfig struct {
	Email     string           `mapstructure:"email"`
	SciName   string           `mapstructure:"sci_name"`
	Password  string           `mapstructure:"password"`
	Assets    []*AdvCashAssets `mapstructure:"assets"`
	assetsMap map[string]string
}

type AdvCashAssets struct {
	PdaxName    string `mapstructure:"pdaxName"`
	AdvCashName string `mapstructure:"advCashName"`
}

func (cfg *AdvCashConfig) GetAssetsMap() map[string]string {
	if cfg.assetsMap == nil {
		list := map[string]string{}
		for _, asset := range cfg.Assets {
			list[asset.PdaxName] = asset.AdvCashName
		}
		cfg.assetsMap = list
	}
	return cfg.assetsMap
}

type LeadBonusConfig map[string]struct {
	Coin    string  `mapstructure:"coin"`
	Amount  float64 `mapstructure:"amount"`
	Comment string  `mapstructure:"comment,omitempty"`
}

// KYBConfig structure
type KYBConfig struct {
	CompanyEmail string `mapstructure:"company_email"`
	ClientID     string `mapstructure:"client_id"`
	SecretKey    string `mapstructure:"secret_key"`
	ShuftiproURL string `mapstructure:"shuftipro_url"`
	CallbackURL  string `mapstructure:"callback_url"`
}

type AWS struct {
	Session Session `mapstructure:"session"`
	Bucket  Bucket  `mapstructure:"bucket"`
}

type Session struct {
	Region      string      `mapstructure:"region"`
	Credentials Credentials `mapstructure:"credentials"`
}

type Credentials struct {
	ID     string `mapstructure:"id"`
	Secret string `mapstructure:"secret"`
	Token  string `mapstructure:"token"`
}

type Bucket struct {
	BucketName   string `mapstructure:"bucket_name_stage"`
	SSEncryption string `mapstructure:"server_side_encryption"`
	StorageClass string `mapstructure:"storage_class"`
}

type OtcDeskConfig struct {
	Url string `mapstructure:"url"`
}

func setDefaultVariables() {
	viper.SetDefault("engine.ids_offset", 10000)
}
