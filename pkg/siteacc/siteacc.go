// Copyright 2018-2023 CERN
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// In applying this license, CERN does not waive the privileges and immunities
// granted to it by virtue of its status as an Intergovernmental Organization
// or submit itself to any jurisdiction.

package siteacc

import (
	"fmt"
	"html"
	"net/http"

	"github.com/cs3org/reva/pkg/siteacc/alerting"
	"github.com/cs3org/reva/pkg/siteacc/config"
	"github.com/cs3org/reva/pkg/siteacc/data"
	acchtml "github.com/cs3org/reva/pkg/siteacc/html"
	"github.com/cs3org/reva/pkg/siteacc/manager"
	accpanel "github.com/cs3org/reva/pkg/siteacc/panels/account"
	"github.com/cs3org/reva/pkg/siteacc/panels/admin"
	"github.com/cs3org/reva/pkg/tracing"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

const tracerName = "siteacc"

// SiteAccounts represents the main Site Accounts service object.
type SiteAccounts struct {
	conf *config.Configuration
	log  *zerolog.Logger

	sessions *acchtml.SessionManager

	storage data.Storage

	operatorsManager *manager.OperatorsManager
	accountsManager  *manager.AccountsManager
	usersManager     *manager.UsersManager

	alertsDispatcher *alerting.Dispatcher

	adminPanel   *admin.Panel
	accountPanel *accpanel.Panel
}

func (siteacc *SiteAccounts) initialize(conf *config.Configuration, log *zerolog.Logger) error {
	if conf == nil {
		return fmt.Errorf("no configuration provided")
	}
	siteacc.conf = conf

	if log == nil {
		return fmt.Errorf("no logger provided")
	}
	siteacc.log = log

	// Create the session mananger
	sessions, err := acchtml.NewSessionManager("siteacc_session", conf, log)
	if err != nil {
		return errors.Wrap(err, "error while creating the session manager")
	}
	siteacc.sessions = sessions

	// Create the central storage
	storage, err := siteacc.createStorage(conf.Storage.Driver)
	if err != nil {
		return errors.Wrap(err, "unable to create storage")
	}
	siteacc.storage = storage

	// Create the sites manager instance
	omngr, err := manager.NewOperatorsManager(storage, conf, log)
	if err != nil {
		return errors.Wrap(err, "error creating the operators manager")
	}
	siteacc.operatorsManager = omngr

	// Create the accounts manager instance
	amngr, err := manager.NewAccountsManager(storage, conf, log)
	if err != nil {
		return errors.Wrap(err, "error creating the accounts manager")
	}
	siteacc.accountsManager = amngr

	// Create the users manager instance
	umngr, err := manager.NewUsersManager(conf, log, siteacc.operatorsManager, siteacc.accountsManager)
	if err != nil {
		return errors.Wrap(err, "error creating the users manager")
	}
	siteacc.usersManager = umngr

	// Create the alerts dispatcher instance
	dispatcher, err := alerting.NewDispatcher(conf, log)
	if err != nil {
		return errors.Wrap(err, "error creating the alerts dispatcher")
	}
	siteacc.alertsDispatcher = dispatcher

	// Create the admin panel
	if pnl, err := admin.NewPanel(conf, log); err == nil {
		siteacc.adminPanel = pnl
	} else {
		return errors.Wrap(err, "unable to create the administration panel")
	}

	// Create the account panel
	if pnl, err := accpanel.NewPanel(conf, log); err == nil {
		siteacc.accountPanel = pnl
	} else {
		return errors.Wrap(err, "unable to create the account panel")
	}

	return nil
}

// RequestHandler returns the HTTP request handler of the service.
func (siteacc *SiteAccounts) RequestHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r, span := tracing.SpanStartFromRequest(r, tracerName, "Siteacc Service HTTP Handler")
		defer span.End()

		defer r.Body.Close()

		// Get the active session for the request (or create a new one); a valid session object will always be returned
		siteacc.sessions.PurgeSessions() // Remove expired sessions first
		session, err := siteacc.sessions.HandleRequest(w, r)
		if err != nil {
			siteacc.log.Err(err).Msg("an error occurred while handling sessions")
		}

		epHandled := false
		for _, ep := range getEndpoints() {
			if ep.Path == r.URL.Path {
				ep.Handler(siteacc, ep, w, r, session)
				epHandled = true
				break
			}
		}

		if !epHandled {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(fmt.Sprintf("Unknown endpoint %v", html.EscapeString(r.URL.Path))))
		}
	})
}

// ShowAdministrationPanel writes the administration panel HTTP output directly to the response writer.
func (siteacc *SiteAccounts) ShowAdministrationPanel(w http.ResponseWriter, r *http.Request, session *acchtml.Session) error {
	// The admin panel only shows the stored accounts and offers actions through links, so let it use cloned data
	accounts := siteacc.accountsManager.CloneAccounts(true)
	operators := siteacc.operatorsManager.CloneOperators(false)
	return siteacc.adminPanel.Execute(w, r, session, &accounts, &operators)
}

// ShowAccountPanel writes the account panel HTTP output directly to the response writer.
func (siteacc *SiteAccounts) ShowAccountPanel(w http.ResponseWriter, r *http.Request, session *acchtml.Session) error {
	return siteacc.accountPanel.Execute(w, r, session)
}

// OperatorsManager returns the central operators manager instance.
func (siteacc *SiteAccounts) OperatorsManager() *manager.OperatorsManager {
	return siteacc.operatorsManager
}

// AccountsManager returns the central accounts manager instance.
func (siteacc *SiteAccounts) AccountsManager() *manager.AccountsManager {
	return siteacc.accountsManager
}

// UsersManager returns the central users manager instance.
func (siteacc *SiteAccounts) UsersManager() *manager.UsersManager {
	return siteacc.usersManager
}

// AlertsDispatcher returns the central alerts dispatcher instance.
func (siteacc *SiteAccounts) AlertsDispatcher() *alerting.Dispatcher {
	return siteacc.alertsDispatcher
}

// GetPublicEndpoints returns a list of all public endpoints.
func (siteacc *SiteAccounts) GetPublicEndpoints() []string {
	// TODO: Only for local testing!
	// return []string{"/"}

	endpoints := make([]string, 0, 5)
	for _, ep := range getEndpoints() {
		if ep.IsPublic {
			endpoints = append(endpoints, ep.Path)
		}
	}
	return endpoints
}

func (siteacc *SiteAccounts) createStorage(driver string) (data.Storage, error) {
	if driver == "file" {
		return data.NewFileStorage(siteacc.conf, siteacc.log)
	}

	return nil, errors.Errorf("unknown storage driver %v", driver)
}

// New returns a new Site Accounts service instance.
func New(conf *config.Configuration, log *zerolog.Logger) (*SiteAccounts, error) {
	// Configure the accounts service
	siteacc := new(SiteAccounts)
	if err := siteacc.initialize(conf, log); err != nil {
		return nil, fmt.Errorf("unable to initialize sites accounts: %v", err)
	}
	return siteacc, nil
}
