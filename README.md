# omnidisk
Omnidisk is a peer-to-peer network designed for secure storage and retrieval
of data. There is no central authority regulating the network, instead it is
designed to provide services while remaining completely trustless and decen-
tralised. There are no defined limits on the size of the data a client wishes to
store nor on the length of time it should be stored for. By leveraging content
identifiers of merkle directed acyclic graphs provided by IPFS, the network
ensures your data is not being tampered with. By providing secure symet-
ric criptography the network ensures only you have access to your data. To
ensure maximum reach the network is capable of NAT traversal and other
Session Traversal Utilities for Nat.


#Brief outline
Every participant in the omnidisk network is called a node, the core func-
tionality of which is built on top of the libp2p package. A node provides
four services which are evoked locally via RPCs: SetOffer, FindCustodians,
RetrieveData and SubscribeToEvents. There are two general types of nodes
participating in the network; those offering storage space and those seeking
storage of their data.

A node is transformed into a storage space offering node by being invoked
by a SetOffer RPC. A node prepared to offer storage space periodi-
cally broadcasts its offer over the network. An offer is a structured message
defining among other things the amount of storage space a node offers and
information on how to contact the node.

To store data the FindCustodians RPC is invoked. Upon joining the
network, a node seeking storage listens for broadcasted offers. After hearing
an offer from a suitable custodian it establishes a point-to-point contact with
the offering node and sends it a contract. A contract is a structured message
specifying, among other things, the data to be stored, how long the data
should be stored and who the owner of the data is. The owner of the data as
specified by a contract shall henceforth be referred to as a contractee whereas
the node that accepts the contract shall be referred to as a contractor. If the
offering node accepts the contract, it starts hosting the data as defined by
the contract. It additionally begins to periodically broadcast the contract it
has accepted over the network.

The contractee can access its data at any time by invoking the Retrieve-
Data RPC. By doing so it again joins the network and starts to listen for
broadcasts. Upon hearing the contract broadcast, it is able to retrieve its
data.

When a node hears a message broadcast or when it is directly contacted
by another node, it is said to experience an event. The SubscribeToEvents
RPC is used to open a stream over which a node forwards its experienced
events.
