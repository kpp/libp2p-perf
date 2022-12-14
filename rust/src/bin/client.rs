use futures::prelude::*;
use libp2p::swarm::SwarmEvent;
use libp2p::{identity, Multiaddr, PeerId, Swarm};
use libp2p_perf::{build_transport, Perf, TransportSecurity};
use structopt::StructOpt;

#[derive(Debug, StructOpt)]
#[structopt(
    name = "libp2p-perf client",
    about = "The iPerf equivalent for the libp2p ecosystem."
)]
struct Opt {
    #[structopt(long)]
    server_address: Multiaddr,

    #[structopt(long)]
    transport_security: Option<TransportSecurity>,
}

#[async_std::main]
async fn main() {
    env_logger::init();
    let opt = Opt::from_args();

    let key = identity::Keypair::generate_ed25519();
    let local_peer_id = PeerId::from(key.public());

    let transport = build_transport(
        false,
        key,
        opt.transport_security.unwrap_or(TransportSecurity::Noise),
    )
    .unwrap();
    let perf = Perf::default();
    let mut client = Swarm::with_async_std_executor(transport, perf, local_peer_id);

    client.dial(opt.server_address).unwrap();

    loop {
        match client.next().await.expect("Infinite stream.") {
            SwarmEvent::Behaviour(e) => {
                println!("{}", e);

                // TODO: Fix hack
                //
                // Performance run timer has already been stopped. Wait for a second
                // to make sure the receiving side of the substream on the server is
                // closed before the whole connection is dropped.
                std::thread::sleep(std::time::Duration::from_secs(1));

                break;
            }
            SwarmEvent::ConnectionEstablished { .. } => {}
            e => panic!("{:?}", e),
        }
    }
}
