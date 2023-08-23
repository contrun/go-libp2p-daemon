package test

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	libp2p "github.com/libp2p/go-libp2p"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/test"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"

	"github.com/stretchr/testify/require"

	golog "github.com/ipfs/go-log/v2"

	cid "github.com/ipfs/go-cid"
	p2pd "github.com/libp2p/go-libp2p-daemon"
	"github.com/libp2p/go-libp2p-daemon/p2pclient"
	pb "github.com/libp2p/go-libp2p-daemon/pb"
	ma "github.com/multiformats/go-multiaddr"
	mh "github.com/multiformats/go-multihash"
)

func createTempDir(t *testing.T) (string, string, func()) {
	root := os.TempDir()
	dir, err := os.MkdirTemp(root, "p2pd")
	if err != nil {
		t.Fatalf("creating temp dir: %s", err)
	}
	daemonPath := filepath.Join(dir, "daemon.sock")
	clientPath := filepath.Join(dir, "client.sock")
	closer := func() {
		os.RemoveAll(dir)
	}
	return daemonPath, clientPath, closer
}

func MustGetPrivateKey(bytes []byte) crypto.PrivKey {
	priv, err := crypto.UnmarshalPrivateKey(bytes)
	if err != nil {
		panic(err)
	}
	return priv
}

func alicePrivateKey() crypto.PrivKey {
	return MustGetPrivateKey(
		[]byte{
			8, 1, 18, 64, 235, 179, 123, 51, 242, 112, 212, 230, 176, 237, 43, 159, 32, 175, 201, 230, 65, 168, 139, 26, 221, 224, 72, 7, 141, 230, 41, 164, 132, 189, 175, 230, 140, 114, 216, 179, 105, 97, 150, 20, 222, 58, 148, 113, 142, 12, 77, 117, 95, 188, 136, 189, 162, 85, 107, 192, 134, 15, 199, 188, 218, 79, 234, 132,
		},
	)
}

func aliceLibp2pOptions() libp2p.Option {
	return libp2p.ChainOptions(
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.ListenAddrStrings(
			"/ip4/0.0.0.0/tcp/33300",
		),
		libp2p.Identity(alicePrivateKey()),
	)
}

func bobPrivateKey() crypto.PrivKey {
	return MustGetPrivateKey(
		[]byte{
			8, 1, 18, 64, 224, 17, 143, 148, 63, 203, 175, 20, 252, 48, 182, 136, 175, 29, 237, 21, 110, 161, 61, 44, 2, 209, 33, 48, 103, 223, 226, 208, 210, 148, 75, 172, 109, 121, 212, 24, 213, 67, 41, 7, 105, 54, 111, 123, 43, 168, 192, 163, 218, 108, 85, 227, 221, 194, 34, 237, 79, 39, 87, 58, 110, 218, 21, 140,
		},
	)
}

func bobLibp2pOptions() libp2p.Option {
	return libp2p.ChainOptions(
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.ListenAddrStrings(
			"/ip4/0.0.0.0/tcp/33301",
		),
		libp2p.Identity(bobPrivateKey()),
	)
}

func libp2pTcpOptions() libp2p.Option {
	var tcpOptions = libp2p.ChainOptions(
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.ListenAddrStrings(
			"/ip4/0.0.0.0/tcp/44444",
		),
	)
	return tcpOptions
}

func createDaemon(t *testing.T, daemonAddr ma.Multiaddr, opts ...libp2p.Option) (*p2pd.Daemon, func()) {
	golog.SetAllLoggers(golog.LevelDebug) // Show all libp2p daemon information
	ctx, cancelCtx := context.WithCancel(context.Background())
	daemon, err := p2pd.NewDaemon(ctx, daemonAddr, "", opts...)
	daemon.EnablePubsub("gossipsub", false, false)
	if err != nil {
		t.Fatal(err)
	}
	return daemon, cancelCtx
}

func createClient(t *testing.T, daemonAddr ma.Multiaddr, clientAddr ma.Multiaddr) (*p2pclient.Client, func()) {
	client, err := p2pclient.NewClient(daemonAddr, clientAddr)
	if err != nil {
		t.Fatal(err)
	}
	closer := func() {
		client.Close()
	}
	return client, closer
}

func createDaemonClientPair(t *testing.T, opts ...libp2p.Option) (*p2pd.Daemon, *p2pclient.Client, func()) {
	dmaddr, cmaddr, dirCloser := getEndpointsMaker(t)(t)
	daemon, closeDaemon := createDaemon(t, dmaddr, opts...)
	client, closeClient := createClient(t, daemon.Listener().Multiaddr(), cmaddr)

	closer := func() {
		closeDaemon()
		closeClient()
		dirCloser()
	}
	return daemon, client, closer
}

type makeEndpoints func(t *testing.T) (daemon, client ma.Multiaddr, cleanup func())

func makeTcpLocalhostEndpoints(t *testing.T) (daemon, client ma.Multiaddr, cleanup func()) {
	daemon, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/0")
	require.NoError(t, err)
	client, err = ma.NewMultiaddr("/ip4/127.0.0.1/tcp/0")
	require.NoError(t, err)
	cleanup = func() {}
	return
}

func makeUnixEndpoints(t *testing.T) (daemon, client ma.Multiaddr, cleanup func()) {
	daemonPath, clientPath, cleanup := createTempDir(t)
	daemon, err := ma.NewComponent("unix", daemonPath)
	require.NoError(t, err)
	client, err = ma.NewComponent("unix", clientPath)
	require.NoError(t, err)
	return
}

func getEndpointsMaker(t *testing.T) makeEndpoints {
	if runtime.GOOS == "windows" {
		return makeTcpLocalhostEndpoints
	} else {
		return makeUnixEndpoints
	}
}

func createMockDaemonClientPair(t *testing.T) (*mockDaemon, *p2pclient.Client, func()) {
	dmaddr, cmaddr, cleanup := getEndpointsMaker(t)(t)

	daemon := newMockDaemon(t, dmaddr, cmaddr)
	client, clientCloser := createClient(t, daemon.listener.Multiaddr(), cmaddr)
	return daemon, client, func() {
		daemon.Close()
		clientCloser()
		cleanup()
	}
}

func randPeerID(t *testing.T) peer.ID {
	id, err := test.RandPeerID()
	if err != nil {
		t.Fatalf("peer id: %s", err)
	}
	return id
}

func randPeerIDs(t *testing.T, n int) []peer.ID {
	ids := make([]peer.ID, n)
	for i := 0; i < n; i++ {
		ids[i] = randPeerID(t)
	}
	return ids
}

func randCid(t *testing.T) cid.Cid {
	buf := make([]byte, 10)
	rand.Read(buf)
	hash, err := mh.Sum(buf, mh.SHA2_256, -1)
	if err != nil {
		t.Fatalf("creating hash for cid: %s", err)
	}
	id := cid.NewCidV1(cid.Raw, hash)
	if err != nil {
		t.Fatalf("creating cid: %s", err)
	}
	return id
}

func randBytes(t *testing.T) []byte {
	buf := make([]byte, 10)
	rand.Read(buf)
	return buf
}

func randPubKey(t *testing.T) crypto.PubKey {
	_, pub, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatalf("generating pubkey: %s", err)
	}
	return pub
}

func wrapDhtResponse(dht *pb.DHTResponse) *pb.Response {
	return &pb.Response{
		Type: pb.Response_OK.Enum(),
		Dht:  dht,
	}
}

func peerInfoResponse(t *testing.T, id peer.ID) *pb.DHTResponse {
	addr, err := ma.NewMultiaddr(fmt.Sprintf("/p2p-circuit/p2p/%s", id.Pretty()))
	if err != nil {
		t.Fatal(err)
	}
	return &pb.DHTResponse{
		Type: pb.DHTResponse_VALUE.Enum(),
		Peer: &pb.PeerInfo{
			Id:    []byte(id),
			Addrs: [][]byte{addr.Bytes()},
		},
	}
}

func peerIDResponse(t *testing.T, id peer.ID) *pb.DHTResponse {
	return &pb.DHTResponse{
		Type:  pb.DHTResponse_VALUE.Enum(),
		Value: []byte(id),
	}
}

func valueResponse(buf []byte) *pb.DHTResponse {
	return &pb.DHTResponse{
		Type:  pb.DHTResponse_VALUE.Enum(),
		Value: buf,
	}
}
