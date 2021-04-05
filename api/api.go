package api

import(
	"context"

	_"github.com/libp2p/go-libp2p-core/peer"
	"go.uber.org/zap"

	apigen "omnidisk/gen/api"
	"omnidisk/node"
)

type Server struct{
	apigen.UnimplementedApiServer

	logger *zap.Logger
	node node.Node
}

func NewServer(logger *zap.Logger, node node.Node) *Server{
	return &Server{
		logger:	logger,
		node:		node,
	}
}


//PING
func (s *Server) Ping(_ context.Context, _ *apigen.PingRequest) (*apigen.PingResponse, error){
	s.logger.Info("handling Ping")

	return &apigen.PingResponse{}, nil
}

//FIND CUSTODIANS
func (s *Server) FindCustodians(_ context.Context, request *apigen.FindCustodiansRequest) (*apigen.FindCustodiansResponse, error){
	s.logger.Info("handling FindCustodians")

	ipfsLink, err := s.node.FindCustodians(request.Filepath, request.Expires, request.Cardinality, request.Datatype)
	if err != nil{
		s.logger.Error("failed finding custodians", zap.Error(err))
		return &apigen.FindCustodiansResponse{IpfsLink: ""}, err
	}

	return &apigen.FindCustodiansResponse{IpfsLink: ipfsLink}, nil
}

//RETRIEVE DATA
func (s *Server) RetrieveData(_ context.Context, request *apigen.RetrieveDataRequest) (*apigen.RetrieveDataResponse, error){

	s.logger.Info("handling RetrieveData")
	ipfsLinks, err := s.node.RetrieveData(request.Datatype, request.Cid, request.Deindex)
	if err != nil{
		s.logger.Error("failed retrieving data", zap.Error(err))
		return &apigen.RetrieveDataResponse{IpfsLinks: ipfsLinks}, err
	}

	return &apigen.RetrieveDataResponse{IpfsLinks: ipfsLinks}, nil
}

//SET OFFER
func (s *Server) SetOffer(_ context.Context, request *apigen.SetOfferRequest) (*apigen.SetOfferResponse, error){
	s.logger.Info("handling SetOffer")

	if err := s.node.SetOffer(request.Capacity); err != nil{
		s.logger.Error("failed setting offer", zap.Error(err))
		return &apigen.SetOfferResponse{Done: false}, err
	}

	return &apigen.SetOfferResponse{Done: true}, nil
}

//SUBSCRIBE TO EVENTS
func (s *Server) SubscribeToEvents(_ *apigen.SubscribeToEventsRequest,
												stream apigen.Api_SubscribeToEventsServer) error {

	s.logger.Info("handling SubscribeToEvents")

	sub, err := s.node.SubscribeToEvents()
	if err != nil{
		return err
	}
	defer sub.Close()

	for{
		evt, err := sub.Next()
		if err != nil{
			return err
		}

		if err := stream.Send(evt.MarshalToProtobuf()); err != nil{
			return err
		}
	}
}
