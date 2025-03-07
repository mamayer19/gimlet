package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"regexp"
	"runtime"
	"time"

	"github.com/gimlet-io/gimlet-cli/cmd/dashboard/config"
	"github.com/gimlet-io/gimlet-cli/cmd/dashboard/dynamicconfig"
	"github.com/gimlet-io/gimlet-cli/pkg/dashboard/model"
	"github.com/gimlet-io/gimlet-cli/pkg/dashboard/notifications"
	"github.com/gimlet-io/gimlet-cli/pkg/dashboard/store"
	"github.com/gimlet-io/gimlet-cli/pkg/git/customScm"
	"github.com/gorilla/securecookie"
	"github.com/sirupsen/logrus"
)

// Creates an admin user, in case there are no users in the database
func setupAdminUser(config *config.Config, store *store.Store) error {
	_, err := store.User("admin")

	if err == sql.ErrNoRows {
		admin := &model.User{
			Login:  "admin",
			Secret: adminToken(config),
			Admin:  true,
		}
		err = store.CreateUser(admin)
		if err != nil {
			return fmt.Errorf("couldn't create user admin user %s", err)
		}

	} else if err != nil {
		return fmt.Errorf("couldn't list users to create admin user %s", err)
	}

	return nil
}

func adminToken(config *config.Config) string {
	if config.AdminToken == "" {
		return base32.StdEncoding.EncodeToString(
			securecookie.GenerateRandomKey(32),
		)
	} else {
		return config.AdminToken
	}
}

func adminKey(dynamicConfig *dynamicconfig.DynamicConfig) string {
	if dynamicConfig.AdminKey == "" {
		adminSecret, _ := randomHex(16)
		dynamicConfig.AdminKey = adminSecret
		dynamicConfig.Persist()

		return adminSecret
	}
	return dynamicConfig.AdminKey
}

func initNotifications(
	config *config.Config,
	dynamicConfig *dynamicconfig.DynamicConfig,
	tokenManager customScm.NonImpersonatedTokenManager,
) *notifications.ManagerImpl {
	notificationsManager := notifications.NewManager()
	if config.Notifications.Provider == "slack" {
		notificationsManager.AddProvider(slackNotificationProvider(config))
	}
	if config.Notifications.Provider == "discord" {
		notificationsManager.AddProvider(discordNotificationProvider(config))
	}
	if dynamicConfig.IsGithub() {
		notificationsManager.AddProvider(notifications.NewGithubProvider(tokenManager))
	} else if dynamicConfig.IsGitlab() {
		notificationsManager.AddProvider(notifications.NewGitlabProvider(tokenManager, dynamicConfig.Gitlab.URL))
	}
	go notificationsManager.Run()
	return notificationsManager
}

// helper function configures the logging.
func initLogger(c *config.Config) {
	logrus.SetReportCaller(true)

	customFormatter := &logrus.TextFormatter{
		CallerPrettyfier: func(f *runtime.Frame) (string, string) {
			filename := path.Base(f.File)
			return "", fmt.Sprintf("[%s:%d]", filename, f.Line)
		},
	}
	customFormatter.FullTimestamp = true
	logrus.SetFormatter(customFormatter)

	if c.Logging.Debug {
		logrus.SetLevel(logrus.DebugLevel)
	}
	if c.Logging.Trace {
		logrus.SetLevel(logrus.TraceLevel)
	}
}

func reencrypt(store *store.Store, encryptionKeyNew string) error {
	if encryptionKeyNew == "" {
		return nil
	}

	users, err := store.EverytingFromUsers()
	if err != nil {
		return err
	}
	for _, u := range users {
		store.UpdateUser(u)
		if err != nil {
			return err
		}
	}

	fmt.Println("db field re-encryption is done, please replace the value of DATABASE_ENCRYPTION_KEY with the value of DATABASE_ENCRYPTION_KEY_NEW, and delete DATABASE_ENCRYPTION_KEY_NEW environment variable")
	os.Exit(0)
	return nil
}

type customFormatter struct{}

func (f *customFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	message := entry.Message
	entry.Message = hideAccessToken(message)

	formatter := &logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: time.RFC3339,
	}
	formatted, err := formatter.Format(entry)

	return []byte(formatted), err
}

func hideAccessToken(message string) string {
	pattern := `access_token=([^& ]+)`
	r, err := regexp.Compile(pattern)
	if err != nil {
		return message
	}
	return r.ReplaceAllString(message, "access_token=***")
}

func generateAndPersistJwtSecret(dynamicConfig *dynamicconfig.DynamicConfig) {
	jwtSecret, _ := randomHex(32)
	dynamicConfig.JWTSecret = jwtSecret
	dynamicConfig.Persist()
}

func randomHex(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
