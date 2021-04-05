package node

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	circuit "github.com/libp2p/go-libp2p-circuit"
	"github.com/libp2p/go-libp2p-core/crypto"
	discovery2 "github.com/libp2p/go-libp2p-core/discovery"
	libp2phost "github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	discovery "github.com/libp2p/go-libp2p-discovery"
	"github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"omnidisk/entities"
	"omnidisk/events"
	"omnidisk/messages"
)

const (
	discoveryNamespace	= "/omnidisk"
	privKeyFileName		= "mojkljuc.privkey"

)

type Node interface{
	//INTERNAL
	ID() peer.ID
	Multiaddr() string

	Start(ctx context.Context, port uint16, pkFilePath string) error
	Bootstrap(ctx context.Context, nodeAddrs []multiaddr.Multiaddr) error
	Shutdown() error

	getPrivKey() (string, error)
	sign(string, []byte) ([]byte, error)
	verify(interface{}) bool

	//RPCS

	FindCustodians(filepath string, expires int64, cardinality, datatype uint32) (string, error)

	RetrieveData(datatype uint32, cid string, deindex bool) ([]string, error)

	SetOffer(capacity uint32) error

	SubscribeToEvents() (events.Subscriber, error)
}

type node struct{
	logger *zap.Logger
	host libp2phost.Host
	kadDHT *dht.IpfsDHT
	ps *pubsub.PubSub

	multiaddr string
	multiaddrLock sync.RWMutex

	privKey crypto.PrivKey

	bootstrapOnly bool

	storeIdentity bool

	connector			*Connector
	contractManager	*ContractManager
	offerManager		*OfferManager
	omniManager			*OmniManager

	eventPublishers		[]events.Publisher
	eventPublishersLock	sync.RWMutex
}


//---------------------------<HELPERS>
func (n *node) ID() peer.ID{
	if n.host == nil{
		return ""
	}
	return n.host.ID()
}

func (n *node) Multiaddr() string{
	if n.host == nil{
		return ""
	}

	return n.multiaddr
}


func (n *node) publishEvent(evt events.Event){
	n.eventPublishersLock.Lock()
	defer n.eventPublishersLock.Unlock()

	//trimming doesn't work (invalid memory error)
	//so for now I'll just comment out the trimmer code
	//var toTrim []int
	for _, pub := range n.eventPublishers{
		if pub.Closed(){
			continue
		} else if err := pub.Publish(evt); err != nil{
			n.logger.Error("failed publishing node event", zap.Error(err))
		//	n.logger.Info("removing this publisher")
		//	toTrim = append(toTrim, ix)
		}
	}

	/*
	//remove error'd (closed) publishers
	if len(toTrim) > 0{
		newPublishers := make([]events.Publisher, len(n.eventPublishers)-len(toTrim))
		toTrim = append(toTrim, -1) //stop marker
		newPublishersIx, toTrimIx := 0, 0
		for ix, p := range n.eventPublishers{
			if ix == toTrim[toTrimIx]{
				toTrimIx++
			} else if toTrim[toTrimIx] == -1{
				break
			} else{
				newPublishers[newPublishersIx] = p
				newPublishersIx++
			}
		}
		n.eventPublishers = newPublishers
	}
	*/
}

func (n *node) joinOmniManagerEvents(sub events.Subscriber){
	for{
		evt, err := sub.Next()
		if err != nil{
			n.logger.Error("failed receiving omni manager event", zap.Error(err))
		}

		n.publishEvent(evt)
	}
}

func (n *node) getPrivateKey(pkFileName string) (crypto.PrivKey, error) {
	var generate bool
	var err error
	var privKeyBytes []byte

	if pkFileName == ""{
		generate = true
	} else{
		privKeyBytes, err = ioutil.ReadFile(pkFileName)
		if os.IsNotExist(err) {
			n.logger.Info("no identity private key file found.", zap.String("pkFileName", pkFileName))
			generate = true
		} else if err != nil {
			return nil, err
		}
	}

	if generate {
		privKey, err := n.generateNewPrivKey()
		if err != nil {
			return nil, err
		}

		privKeyBytes, err := crypto.MarshalPrivateKey(privKey)
		if err != nil {
			return nil, errors.Wrap(err, "marshalling identity private key")
		}

		f, err := os.Create(privKeyFileName)
		if err != nil {
			return nil, errors.Wrap(err, "creating identity private key file")
		}
		defer f.Close()

		if _, err := f.Write(privKeyBytes); err != nil {
			return nil, errors.Wrap(err, "writing identity private key to file")
		}

		return privKey, nil
	}

	privKey, err := crypto.UnmarshalPrivateKey(privKeyBytes)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshalling identity private key")
	}

	n.logger.Info("loaded identity private key from file")
	return privKey, nil
}

func (n *node) generateNewPrivKey() (crypto.PrivKey, error) {
	n.logger.Info("generating identity private key")
	privKey, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, errors.Wrap(err, "generating identity private key")
	}
	n.logger.Info("generated new identity private key")

	return privKey, nil
}

func (n *node) subscribeToEvents() (events.Subscriber, error){
	pub, sub := events.NewSubscription()
	n.eventPublishersLock.Lock()
	defer n.eventPublishersLock.Unlock()
	n.eventPublishers = append(n.eventPublishers, pub)

	return sub, nil
}

func (n *node) getPrivKey() (string, error){
	rawBytes, err := n.privKey.Raw()
	if err != nil{
		return "", err
	}
	return base64.StdEncoding.EncodeToString(rawBytes), nil
}
//---------------------------</HELPERS>
//---------------------------<SETUP>

func NewNode(logger *zap.Logger, bootstrapOnly bool, storeIdentity bool) Node{
	if logger == nil{
		logger = zap.NewNop()
	}

	return &node{
		logger:			logger,
		host:				nil,
		bootstrapOnly:	bootstrapOnly,
		storeIdentity:	storeIdentity,
	}
}

func (n *node) Start(ctx context.Context, port uint16, pkFileName string) error{
	n.logger.Info("starting node", zap.Bool("bootstrapOnly", n.bootstrapOnly))

	nodeAddrStrings := []string{fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port)}

	privKey, err := n.getPrivateKey(pkFileName)
	if err != nil {
		return err
	}

	n.logger.Debug("creating libp2p host")
	host, err := libp2p.New(
		ctx,
		libp2p.ListenAddrStrings(nodeAddrStrings...),
		libp2p.Identity(privKey),
//		libp2p.EnableAutoRelay(),
		libp2p.EnableNATService(),
		libp2p.EnableRelay(),
		libp2p.NATPortMap(),
		libp2p.EnableRelay(circuit.OptHop),
//		libp2p.EnableRelay(circuit.OptDiscovery),
//		libp2p.EnableRelay(),
	)
	if err != nil{
		return errors.Wrap(err, "creating libp2p host")
	}
	n.host = host
	n.privKey = privKey

	n.logger.Debug("creating pubsub")
	ps, err := pubsub.NewGossipSub(ctx, n.host, pubsub.WithMessageSignaturePolicy(pubsub.StrictSign))
	if err != nil{
		return errors.Wrap(err, "creating pubsub")
	}
	n.ps = ps

	p2pAddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/p2p/%s", host.ID().Pretty()))
	if err != nil {
		return errors.Wrap(err, "creating host p2p multiaddr")
	}

	var fullAddrs []string
	for _, addr := range host.Addrs() {
		fullAddrs = append(fullAddrs, addr.Encapsulate(p2pAddr).String())
	}
	n.multiaddr = fullAddrs[0]

	n.logger.Info("started node", zap.Strings("p2pAddresses", fullAddrs))

	if n.bootstrapOnly{
		return nil
	}

	n.logger.Debug("creating OfferManager")
	offerManager := NewOfferManager(n.logger, n)
	n.offerManager = offerManager

	n.logger.Debug("creating Connector")
	connector, connectorEvtSub := NewConnector(n.logger, n, n.host, n.offerManager)
	n.connector = connector
	go n.joinOmniManagerEvents(connectorEvtSub)

	n.logger.Debug("creating ContractManager")
	contractManager := NewContractManager(n.logger, n, n.connector)
	n.contractManager = contractManager

	return nil
}


func (n *node) Bootstrap(ctx context.Context, nodeAddrs []multiaddr.Multiaddr) error{
	var bootstrappers []peer.AddrInfo
	for _, nodeAddr := range nodeAddrs{
		pi, err := peer.AddrInfoFromP2pAddr(nodeAddr)
		if err != nil{
			return errors.Wrap(err, "parsing bootstrapper node address info from p2p address")
		}

		bootstrappers = append(bootstrappers, *pi)
	}

	n.logger.Debug("creating routing DHT")
	kadDHT, err := dht.New(
		ctx,
		n.host,
		dht.BootstrapPeers(bootstrappers...),
		dht.ProtocolPrefix(discoveryNamespace),
		dht.Mode(dht.ModeAutoServer),
	)
	if err != nil{
		return errors.Wrap(err, "creating routing DHT")
	}
	n.kadDHT = kadDHT

	if err := kadDHT.Bootstrap(ctx); err != nil{
		return errors.Wrap(err, "bootstrapping DHT")
	}

	if n.bootstrapOnly{
		return nil
	}

	n.logger.Debug("setting up OmniManager")
	omniManager, omniManagerEvtSub := NewOmniManager(n.logger, n, n.kadDHT, n.ps, n.contractManager, n.offerManager)
	n.omniManager = omniManager
	n.omniManager.JoinOmnidisk()
	go n.joinOmniManagerEvents(omniManagerEvtSub)

	if len(nodeAddrs) == 0{
		return nil
	}

	//connect to bootstrap nodes
	for _,pi := range bootstrappers{
		if err := n.host.Connect(ctx, pi); err != nil {
			return errors.Wrap(err, "connecting to bootstrap node")
		}
	}

	rd := discovery.NewRoutingDiscovery(kadDHT)

	n.logger.Info("starting advertising thread")
	discovery.Advertise(ctx, rd, discoveryNamespace)

	//try finding more peers
	go func(){
		printOnce := false
		for{
			//n.logger.Info("looking for peers...")

			peersChan, err := rd.FindPeers(
				ctx,
				discoveryNamespace,
				discovery2.Limit(100),
			)
			if err != nil{
				n.logger.Error("failed trying to find peers", zap.Error(err))
				continue
			}

			//read all channel messages to avoid blocking the peer query
			for range peersChan{
			}

		/*
			n.logger.Info("done looking for peers",
				zap.Int("peerCount", n.host.Peerstore().Peers().Len()),
			)
		*/

		//	fmt.Printf("\n\nhost addrs: %v\n\n", n.host.Addrs())
		//	fmt.Printf("\n\nmy multiaddr: %v\n\n", n.Multiaddr())
			addrs := n.host.Addrs()
			if len(addrs) > 2{
				n.multiaddrLock.RLock()
				n.multiaddr = addrs[len(addrs) - 1].String() + "/p2p/" + n.ID().Pretty() //last one is public addr
				n.multiaddrLock.RUnlock()

				if !printOnce{
					n.logger.Info("done looking for peers",
						zap.Int("peerCount", n.host.Peerstore().Peers().Len()),
					)
					fmt.Printf("\n\nmy multiaddr: %v\n\n", n.Multiaddr())
					printOnce = true
				}
			}
		/*
		*/

			<-time.After(time.Minute)
		}
	}()

	return nil
}


func (n *node) Shutdown() error{
	return n.host.Close()
}

//---------------------------</SETUP>
//---------------------------<RPC>
func (n *node) FindCustodians(filepath string, expires int64, cardinality, datatype uint32) (string, error){
	if n.bootstrapOnly{
		return "", errors.New("can't find custodians on a bootstrap-only node")
	}

	//prepare contract
	n.logger.Info("preparing contract..")
	contract, err := n.contractManager.PrepareContract(filepath, expires, cardinality, datatype)
	if err != nil{
		return "", errors.New("preparing contract: FAILED")
	}
	n.logger.Info("preparing contract: DONE")

	n.logger.Info("subscribing to events..")
	sub, err := n.subscribeToEvents()
	if err != nil{
		n.logger.Error("subscribing to events: FAILED", zap.Error(err))
		return "", errors.New("failed to subscribe to events")
	}
	n.logger.Info("subscribing to events: DONE")

	//seek storage from offering nodes
	n.logger.Info("seeking storage")
	ipfsLink, err := n.contractManager.SeekStorage(contract, sub)
	if err != nil{
		return "", errors.New("seeking storage: FAILED")
	}
	n.logger.Info("seeking storage: DONE!")
	n.logger.Info("DON'T LOSE THE FILE WITH YOUR PRIVATE KEY", zap.String("privKeyFileName", privKeyFileName))

	sub.Close() //close the subscriber

	return ipfsLink, nil
}

func (n *node) sign(ownerID string, msg []byte) ([]byte, error){
	//XXX IMPORTING OWNER_ID FROM FILE TO SIGN?
	if ownerID != n.ID().Pretty(){
		n.logger.Warn("cannot sign content of foreign owner")
		return make([]byte, 0), errors.New("ownerID does not match node ID")
	}

	return n.privKey.Sign(msg)

/*
	pubkey1 := n.privKey.GetPublic()
	fmt.Printf("pubkey1 matches pubkey from id:  %v\n\n", n.ID().MatchesPublicKey(pubkey1))
	tru, _ := pubkey1.Verify(msg, signature)
	fmt.Printf("pubkey1 says signature is:  %v\n\n", tru)

	pubkey2, _ := n.ID().ExtractPublicKey()
	fmt.Printf("pubkey2 matches pubkey from id:  %v\n\n", n.ID().MatchesPublicKey(pubkey2))
	tru, _ = pubkey2.Verify(msg, signature)
	fmt.Printf("pubkey2 says signature is:  %v\n\n", tru)

	fmt.Printf("pubkey1 equals pubkey2:  %v\n\n", pubkey1.Equals(pubkey2))

	return signature, nil
*/
}

func (n *node) verify(message interface{}) bool{
	switch msg := message.(type){
		case messages.RetrievalRequest:
			ownerID, err := peer.Decode(msg.OwnerID)
			if err != nil{
				n.logger.Debug("cannot verify message with invalid owner ID")
				return false
			}
			pubKey, err := ownerID.ExtractPublicKey()
			if err != nil{
				n.logger.Debug("pubkey extraction from owner ID failed")
				return false
			}

			str := msg.Multiaddr + msg.OwnerID + strconv.FormatInt(msg.Timestamp.Unix(), 10)
			ver, err := pubKey.Verify([]byte(str), []byte(msg.Signature))
			if err != nil{
				n.logger.Debug("error during signature verification")
				return false
			}
			return ver

		case entities.Offer:
			components := strings.Split(msg.Multiaddr, "/")
			ownerID, err := peer.Decode(components[len(components)-1])
			fmt.Printf("\n\nthis is ownerID:   '%v'\n\n", ownerID)
			if err != nil{
				n.logger.Debug("cannot verify message with invalid owner ID")
				return false
			}
			pubKey, err := ownerID.ExtractPublicKey()
			if err != nil{
				n.logger.Debug("pubkey extraction from owner ID failed")
				return false
			}

			str := msg.Multiaddr + strconv.FormatUint(uint64(msg.Capacity), 10) + strconv.FormatInt(msg.Timestamp.Unix(), 10)
			str = "kek"
			fmt.Printf("\n\nthis is offer:  %v\n\n", msg)
			fmt.Printf("\n\nthis is offer str:  %v\n\n", str)
			fmt.Printf("\n\nthis is offer signature:  %v\n\n", msg.Signature)
			ver, err := pubKey.Verify([]byte(str), []byte(msg.Signature))
			if err != nil{
				n.logger.Debug("error during signature verification")
				return false
			}
			n.logger.Debug("okej ampak zdej sm tuki")
			fmt.Printf("picku mater nooooo     %v\n\n", ver)
			return ver

		default:
			n.logger.Debug("cannot verify unknown message type")
			return false
	}
}

func (n *node) RetrieveData(datatype uint32, cid string, deindex bool) ([]string, error){
	if n.bootstrapOnly{
		return make([]string, 0), errors.New("can't retrieve data from bootstrap node")
	}

/*
	//prepare retrieval request
	n.logger.Info("preparing retrieval request")
	ownerID := n.ID().Pretty()
	request, err := n.contractManager.PrepareRetrievalRequest(ownerID)
	if err != nil{
		return make([]string, 0), errors.New("preparing retrieval request: FAILED")
	}
	n.logger.Info("preparing retrieval request: DONE")

	sub, err := n.subscribeToEvents()
	if err != nil{
		return make([]string, 0), errors.New("failed to subscribe to events")
	}

	n.logger.Info("retrieving data")
	ipfsLinks, err := n.contractManager.RetrieveData(request, sub)
	if err != nil{
		return make([]string, 0), errors.New("retrieving data: FAILED")
	}
	n.logger.Info("retrieving data: DONE")
*/

	n.logger.Info("subscribing to events..")
	sub, err := n.subscribeToEvents()
	if err != nil{
		n.logger.Error("subscribing to events: FAILED", zap.Error(err))
		return make([]string, 0), errors.New("failed to subscribe to events")
	}
	n.logger.Info("subscribing to events: DONE")

	n.logger.Info("retrieving data..")
	ipfsLinks, err := n.contractManager.RetrieveDataV2(datatype, cid, deindex, sub)
	if err != nil{
		n.logger.Error("retrieving data: FAILED", zap.Error(err))
		return make([]string, 0), errors.New("retrieving data: FAILED")
	}
	n.logger.Info("retrieving data: DONE")

	sub.Close() //close the subscriber
	return ipfsLinks, nil
}

func (n *node) SetOffer(capacity uint32) error{
	if n.bootstrapOnly{
		return errors.New("can't set offer on a bootstrap-only node")
	}

	if err := n.omniManager.SetOffer(capacity); err != nil{
		return err
	}
	return nil
}


func (n *node) SubscribeToEvents() (events.Subscriber, error){
	if n.bootstrapOnly{
		return nil, errors.New("can't subscribe to omni events on a bootstrap-only node")
	}

	return n.subscribeToEvents()
}




//---------------------------</RPC>

/*
*/


