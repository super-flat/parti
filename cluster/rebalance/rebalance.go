package rebalance

import (
	"sort"
)

type rebalancePeer struct {
	PeerID       string
	PartitionIDs *queue[uint32]
}

// ComputeRebalance returns the ideal balance for mapping partitions to nodes assuming
// all partitions are equal volume
func ComputeRebalance(numPartitions uint32, currentAssignments map[uint32]string, activePeerIDs []string) map[uint32]string {
	if len(activePeerIDs) == 0 {
		return nil
	}

	// compute the number of assignments per peer
	peerMap := make(map[string]*rebalancePeer, len(activePeerIDs))
	for _, peerID := range activePeerIDs {
		peerMap[peerID] = &rebalancePeer{
			PeerID:       peerID,
			PartitionIDs: newQueue[uint32]((numPartitions)),
		}
	}

	// track which partitions are on which active peers
	unassigned := newQueue[uint32](numPartitions)
	// enumerate partitions and add to peer queues, else add to unassigned queue
	for partitionID := uint32(0); partitionID < numPartitions; partitionID++ {
		if peerID, isAssigned := currentAssignments[partitionID]; isAssigned {
			if peer, exists := peerMap[peerID]; exists {
				peer.PartitionIDs.Enqueue(partitionID)
				continue
			}
		}
		unassigned.Enqueue(partitionID)
	}

	// sort the peers according to number of assignments
	peers := make([]*rebalancePeer, 0, len(peerMap))
	for _, peer := range peerMap {
		peers = append(peers, peer)
	}
	sort.SliceStable(peers, func(i, j int) bool {
		return peers[i].PartitionIDs.Length() < peers[j].PartitionIDs.Length()
	})

	// assign all unassigned values to last peer
	for {
		value, ok := unassigned.Dequeue()
		if !ok {
			break
		}
		peers[len(peers)-1].PartitionIDs.Enqueue(value)
	}

	i := 0
	j := len(peers) - 1

	idealBalance := int(numPartitions) / len(peers)

	for i != j {
		lowPeer := peers[i]
		highPeer := peers[j]
		// if the lower peer has the ideal number of partitions, continue
		if lowPeer.PartitionIDs.Length() >= idealBalance {
			i += 1
			continue
		}
		// let the heaviest peers have more partitions than the
		// lightest until you cross the "remainder"
		maxBalance := idealBalance
		if len(peers)-j <= int(numPartitions)%len(peers) {
			maxBalance += 1
		}
		// if the highest peer has the ideal number of partitions, continue
		if highPeer.PartitionIDs.Length() == maxBalance {
			j -= 1
			continue
		}
		// take a partition from the highest peer and give to the lowest peer
		if assignment, ok := highPeer.PartitionIDs.Dequeue(); ok {
			lowPeer.PartitionIDs.Enqueue(assignment)
		}
	}

	output := make(map[uint32]string, numPartitions)

	for _, peer := range peers {
		for {
			if partitionID, ok := peer.PartitionIDs.Dequeue(); ok {
				output[partitionID] = peer.PeerID
			} else {
				break
			}
		}
	}

	return output
}
