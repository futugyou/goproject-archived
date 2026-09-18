package mqtt

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"

	"github.com/eclipse/paho.golang/paho"
	"github.com/futugyou/openclaw/core"
)

func CreateMqttClient(
	ctx context.Context,
	config core.MqttConfig,
	receiveHandlers []func(paho.PublishReceived) (bool, error),
	errorHandler func(err error),
) (*paho.Client, error) {

	username := core.SecretResolverInstance.Resolve(config.UsernameRef)
	password := core.SecretResolverInstance.Resolve(config.PasswordRef)

	conn, err := tls.Dial("tcp", net.JoinHostPort(config.Host, fmt.Sprintf("%d", config.Port)), &tls.Config{
		ServerName: config.Host, // Must pass SNI
	})
	if err != nil {
		return nil, err
	}

	clientid := config.ClientId
	if clientid == "" {
		clientid = "openclaw"
	}

	clientConfig := paho.ClientConfig{
		ClientID: clientid,
		Conn:     conn,
		OnClientError: func(err error) {
			fmt.Printf("client error: %v\n", err)
		},
		OnServerDisconnect: func(d *paho.Disconnect) {
			fmt.Printf("server disconnect, reason code: %d\n", d.ReasonCode)
		},
	}

	if len(receiveHandlers) > 0 {
		clientConfig.OnPublishReceived = receiveHandlers
	}

	if errorHandler != nil {
		clientConfig.OnClientError = errorHandler
	}

	client := paho.NewClient(clientConfig)

	connectProperties := &paho.Connect{
		ClientID:   clientid,
		KeepAlive:  30,
		CleanStart: true,
		Username:   username,
		Password:   []byte(password),
		Properties: &paho.ConnectProperties{
			User: []paho.UserProperty{
				{Key: "env", Value: "production"},
			},
		},
	}

	connack, err := client.Connect(ctx, connectProperties)
	if err != nil {
		return nil, err
	}

	if connack.ReasonCode != 0 {
		return nil, fmt.Errorf("conn rejected, reason code: %d", connack.ReasonCode)
	}
	return client, nil
}
