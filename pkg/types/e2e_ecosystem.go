package types

// EcosystemCommon is the configuration every data service shares: where the
// instance lives, how it authenticates, and how big it is.
//
// It is embedded by each dependency type.
type EcosystemCommon struct {
	// Namespace for the instance. Default: "default".
	Namespace string `yaml:"namespace,omitempty"`
	// User is the application role. Default: "app", with login: true.
	User string `yaml:"user,omitempty"`
	// Database is owned by the role. Default: same as User.
	Database string `yaml:"database,omitempty"`
	// Password is the role's or instance's password. Omitted: the chart's
	// default.
	Password string `yaml:"password,omitempty"`
	// Storage is the PVC size. Omitted: the chart's default.
	Storage string `yaml:"storage,omitempty"`
	// Replicas is the replica count. Default: 1.
	Replicas int `yaml:"replicas,omitempty"`
}

// Ecosystem declares the infrastructure dependencies a run needs and the
// Secrets that carry their connection details to the workload.
//
// Each dependency resolves onto the data-services chart: the chart installs
// the operator, provisions the instance, and waits for readiness. The
// declaration is an overlay — only the fields it sets differ from the
// chart's defaults.
type Ecosystem struct {
	// Postgres, RabbitMQ, Kafka, MySQL, Redis declare dependencies. Each is
	// optional; the chart installs only what is declared.
	Postgres *EcosystemPostgres `yaml:"postgres,omitempty"`
	Rabbitmq *EcosystemRabbitmq `yaml:"rabbitmq,omitempty"`
	Kafka    *EcosystemKafka    `yaml:"kafka,omitempty"`
	Mysql    *EcosystemMysql    `yaml:"mysql,omitempty"`
	Redis    *EcosystemRedis    `yaml:"redis,omitempty"`

	// Secrets declares how the resolved connection details reach the
	// workload: export writes the tool's canonical variables, define writes
	// Secrets the test names with the app's own keys.
	Secrets *EcosystemSecrets `yaml:"secrets,omitempty"`
}

// EcosystemPostgres declares a Postgres instance.
type EcosystemPostgres struct {
	EcosystemCommon `yaml:",inline"`
}

// EcosystemMysql declares a MySQL instance.
type EcosystemMysql struct {
	EcosystemCommon `yaml:",inline"`
}

// EcosystemRabbitmq declares a RabbitMQ instance.
type EcosystemRabbitmq struct {
	EcosystemCommon `yaml:",inline"`
	// Vhosts lists the vhosts to create. Default: [default].
	Vhosts []string `yaml:"vhosts,omitempty"`
}

// EcosystemRedis declares a Redis instance.
type EcosystemRedis struct {
	EcosystemCommon `yaml:",inline"`
}

// EcosystemKafka declares a Kafka instance.
type EcosystemKafka struct {
	EcosystemCommon `yaml:",inline"`
	// Topics lists the topics to create.
	Topics []string `yaml:"topics,omitempty"`
}

// EcosystemSecrets declares how dependency connection details reach the
// workload. export and define are independent and composable in one spec.
type EcosystemSecrets struct {
	// Export writes the tool's canonical variables (POSTGRES_URL, REDIS_HOST,
	// …) for the named tools into one Secret.
	Export *EcosystemExport `yaml:"export,omitempty"`

	// Define writes Secrets the test names, with the app's own keys and
	// resolver paths.
	Define map[string]EcosystemDefine `yaml:"define,omitempty"`
}

// EcosystemExport writes the canonical variables for the named tools.
type EcosystemExport struct {
	// Tools names the dependencies to export. Omitted: all declared.
	Tools []string `yaml:"tools,omitempty"`
	// Name is the Secret name. Default: "inrun-ecosystem".
	Name string `yaml:"name,omitempty"`
	// Namespace is the Secret namespace. Default: the spec's default namespace.
	Namespace string `yaml:"namespace,omitempty"`
}

// EcosystemDefine is a Secret the test names.
type EcosystemDefine struct {
	// Namespace is the Secret namespace. Default: the spec's default namespace.
	Namespace string `yaml:"namespace,omitempty"`
	// Keys maps the app's env var names to resolver paths:
	//
	//	keys:
	//	  POSTGRES_HOST_PROD: postgres.host
	//	  KAFKA_TOPIC: kafka.topics[0]
	Keys map[string]string `yaml:"keys"`
}
