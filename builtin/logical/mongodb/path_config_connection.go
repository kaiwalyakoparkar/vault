package mongodb

import (
	"fmt"

	"github.com/fatih/structs"
	"github.com/hashicorp/vault/logical"
	"github.com/hashicorp/vault/logical/framework"
	"gopkg.in/mgo.v2"
)

func pathConfigConnection(b *backend) *framework.Path {
	return &framework.Path{
		Pattern: "config/connection",
		Fields: map[string]*framework.FieldSchema{
			"uri": &framework.FieldSchema{
				Type:        framework.TypeString,
				Description: "MongoDB standard connection string (URI)",
			},
			"verify_connection": &framework.FieldSchema{
				Type:        framework.TypeBool,
				Default:     true,
				Description: `If set, uri is verified by actually connecting to the database`,
			},
		},
		Callbacks: map[logical.Operation]framework.OperationFunc{
			logical.ReadOperation:   b.pathConnectionRead,
			logical.UpdateOperation: b.pathConnectionWrite,
		},
		HelpSynopsis:    pathConfigConnectionHelpSyn,
		HelpDescription: pathConfigConnectionHelpDesc,
	}
}

// pathConnectionRead reads out the connection configuration
func (b *backend) pathConnectionRead(req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	entry, err := req.Storage.Get("config/connection")
	if err != nil {
		return nil, fmt.Errorf("failed to read connection configuration")
	}
	if entry == nil {
		return nil, nil
	}

	var config connectionConfig
	if err := entry.DecodeJSON(&config); err != nil {
		return nil, err
	}
	return &logical.Response{
		Data: structs.New(config).Map(),
	}, nil
}

func (b *backend) pathConnectionWrite(req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	uri := data.Get("uri").(string)
	if uri == "" {
		return logical.ErrorResponse("uri parameter must be supplied"), nil
	}

	dialInfo, err := parseMongoURI(uri)
	if err != nil {
		return logical.ErrorResponse(fmt.Sprintf("invalid uri: %s", err)), nil
	}

	// Don't check the config if verification is disabled
	verifyConnection := data.Get("verify_connection").(bool)
	if verifyConnection {
		// Verify the config
		session, err := mgo.DialWithInfo(dialInfo)
		if err != nil {
			return logical.ErrorResponse(fmt.Sprintf(
				"Error validating connection info: %s", err)), nil
		}
		defer session.Close()
		if err := session.Ping(); err != nil {
			return logical.ErrorResponse(fmt.Sprintf(
				"Error validating connection info: %s", err)), nil
		}
	}

	// Store it
	entry, err := logical.StorageEntryJSON("config/connection", connectionConfig{
		URI: uri,
		VerifyConnection: verifyConnection,
	})
	if err != nil {
		return nil, err
	}
	if err := req.Storage.Put(entry); err != nil {
		return nil, err
	}

	// Reset the Session
	b.ResetSession()

	resp := &logical.Response{}
	resp.AddWarning("Read access to this endpoint should be controlled via ACLs as it will return the connection URI as it is, including passwords, if any.")

	return resp, nil
}

type connectionConfig struct {
	URI              string `json:"uri" structs:"uri" mapstructure:"uri"`
	VerifyConnection bool   `json:"verify_connection" structs:"verify_connection" mapstructure:"verify_connection"`
}

const pathConfigConnectionHelpSyn = `
Configure the connection string to talk to MongoDB.
`

const pathConfigConnectionHelpDesc = `
This path configures the standard connection string (URI) used to connect to MongoDB.

A MongoDB URI looks like:
"mongodb://[username:password@]host1[:port1][,host2[:port2],...[,hostN[:portN]]][/[database][?options]]"

See https://docs.mongodb.org/manual/reference/connection-string/ for detailed documentation of the URI format.

When configuring the connection string, the backend will verify its validity.
`
