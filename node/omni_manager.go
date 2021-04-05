package node

import (
	"context"
	"encoding/json"
	_"math"
	"sync"
	"time"

	_"github.com/libp2p/go-libp2p-core/peer"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"omnidisk/entities"
	"omnidisk/events"
)

const (
	topicName = "omnidisk_55488"
)


type OmniMessageType string
const (
	//this is published to give info on all contracts of this node
	OmniMessageTypeContractPub OmniMessageType = "pub.contract"
	//this is published to give info on the storage offer of this node
	OmniMessageTypeOfferPub OmniMessageType = "pub.offer"
)

//holds data to be published on the omnidisk topic
type OmniMessageOut struct {
	Type OmniMessageType	`json:"type"`
	Payload	interface{}	`json:"payload,omitempty"`
}

//holds data to be received from the omnidisk topic
type OmniMessageIn struct{
	Type OmniMessageType		`json:"type"`
	Payload json.RawMessage	`json:"payload,omitempty"`
}


//OmniManager manages omnidisk traffic through the pubsub and implements operations
type OmniManager struct{
	logger	*zap.Logger
	node		Node
	kadDHT	*dht.IpfsDHT

	ps					*pubsub.PubSub
	subscription	*pubsub.Subscription
	topic				*pubsub.Topic

	contractManager	*ContractManager
	offerManager		*OfferManager

	eventPublisher events.Publisher

	lock sync.RWMutex
}

func NewOmniManager(logger *zap.Logger, node Node, kadDHT *dht.IpfsDHT, ps *pubsub.PubSub, contractManager *ContractManager, offerManager *OfferManager) (*OmniManager, events.Subscriber){
	if logger == nil{
		logger = zap.NewNop()
	}

	evtPub, evtSub := events.NewSubscription()

	mngr := &OmniManager{
		logger:				logger,
		ps:					ps,
		node:					node,
		kadDHT:				kadDHT,
		contractManager:	contractManager,
		offerManager:		offerManager,
		eventPublisher:	evtPub,
	}

	return mngr, evtSub
}

func (om *OmniManager) JoinOmnidisk() error{

	logger := om.logger.With(zap.String("topic", topicName))

	cleanup := func(topic *pubsub.Topic, subscription *pubsub.Subscription){
		if topic != nil{
			_ = topic.Close()
		}
		if subscription != nil{
			subscription.Cancel()
		}
	}

	logger.Debug("joining omnidisk topic")
	topic, err := om.ps.Join(topicName)
	if err != nil{
		logger.Debug("failed joining omnidisk topic")
		return err
	}

	logger.Debug("subscribing to omnidisk topic")
	subscription, err := topic.Subscribe()
	if err != nil{
		logger.Debug("failed subscribing to omnidisk topic")

		cleanup(topic, subscription)
		return err
	}

	om.topic = topic
	om.subscription = subscription
	go om.omniSubscriptionHandler()

	logger.Debug("successfuly joined omnidisk")

	om.logger.Info("starting contract advertising")
	go om.advertiseContracts()

	return nil
}


func (om *OmniManager) omniSubscriptionHandler(){
	for{
		subMsg, err := om.subscription.Next(context.Background())
		if err != nil{
			om.logger.Error("failed receiving omnidisk subscription message", zap.Error(err))
		}

		if subMsg.ReceivedFrom == om.node.ID(){
			continue
		}

		var omi OmniMessageIn
		if err := json.Unmarshal(subMsg.Data, &omi); err != nil{
			om.logger.Warn("cannot unmarshal omni message. Ignoring", zap.Error(err))
			continue
		}

		switch omi.Type{
			case OmniMessageTypeOfferPub:
				var offer entities.Offer
				if err := json.Unmarshal(omi.Payload, &offer); err != nil{
					om.logger.Warn("ignoring offer pub",
						zap.Error(errors.Wrap(err, "unmarshalling payload")),
					)
					continue
				}

				/*
				if ver := om.node.verify(offer); ver != true{
					om.logger.Debug("could not verify published offer; ignoring")
					continue
				}
				*/
				expires := offer.Timestamp.Add(3 * time.Minute)
				if expires.Before(time.Now()){
					om.logger.Debug("published offer expired; ignoring")
				}

				if err := om.eventPublisher.Publish(&events.OfferPub{Offer: offer}); err != nil{
					om.logger.Error("failed publishing omni manager event", zap.Error(err))
				}

			case OmniMessageTypeContractPub:
				type payload struct{
					Multiaddr string						`json:"multiaddr"`
					Contracts []entities.Contract		`json:"contracts"`
				}

				var pld payload

				if err := json.Unmarshal(omi.Payload, &pld); err != nil{
					om.logger.Warn("ignoring contract pub",
						zap.Error(errors.Wrap(err, "unmarshalling payload")),
					)
					continue
				}

				err := om.eventPublisher.Publish(&events.ContractPub{
																						Multiaddr:	pld.Multiaddr,
																						Contracts:	pld.Contracts,
																						})
				if err != nil{
					om.logger.Error("failed publishing omni manager event", zap.Error(err))
				}

			default:
				om.logger.Warn("ignoring omni message",
					zap.Error(errors.New("unknown omni message type")),
				)
		}
	}
}

//---------RPC
func (om *OmniManager) SetOffer(capacity uint32) error{
	if err := om.offerManager.SetOffer(capacity); err != nil{
		return err
	}

	om.logger.Info("starting offer advertising")
	go om.advertiseOffer()

	return nil
}


//---------MESSAGING
func (om *OmniManager) publishOmniMessage(ctx context.Context, omo *OmniMessageOut) error{
	omoJSON, err := json.Marshal(omo)
	if err != nil{
		return errors.Wrap(err, "marshalling omni message")
	}

	if err := om.topic.Publish(ctx, omoJSON); err != nil{
		return err
	}

	return nil
}


func (om *OmniManager) advertiseOffer(){
	if !om.offerManager.IsOfferSet(){
		om.logger.Warn("must set offfer before publishing it")
	}

	tick := time.Tick(23 * time.Second)

	for{
		<-tick

		func(){
			om.offerManager.Lock()
			defer om.offerManager.Unlock()

			offer := om.offerManager.GetOffer(true)
			omo := OmniMessageOut{
				Type:		OmniMessageTypeOfferPub,
				Payload:	offer,
			}
			if err := om.publishOmniMessage(context.Background(), &omo); err != nil {
				om.logger.Error("failed publishing offer", zap.Error(err))
			}
		}()
	}
}

func (om *OmniManager) advertiseContracts(){
		tick := time.Tick(23 * time.Second)

		type payload struct{
			Multiaddr string						`json:"multiaddr"`
			Contracts []entities.Contract		`json:"contracts"`
		}

		for{
			<-tick

			func(){
				om.offerManager.Lock()
				defer om.offerManager.Unlock()

				contracts := om.offerManager.GetContracts()
				if len(contracts) == 0{
					return
				}

				pld := payload{
					Multiaddr:		om.node.Multiaddr(),
					Contracts:		contracts,
				}
/*
*/
				omo := OmniMessageOut{
					Type:		OmniMessageTypeContractPub,
					Payload:	pld,
				}
				if err := om.publishOmniMessage(context.Background(), &omo); err != nil{
					om.logger.Error("failed publishing contracts", zap.Error(err))
				}
			}()
		}
}

