package gov

import (
	"github.com/PlatONnetwork/PlatON-Go/common"
	"github.com/PlatONnetwork/PlatON-Go/core/snapshotdb"
	"github.com/PlatONnetwork/PlatON-Go/p2p/discover"
	"github.com/PlatONnetwork/PlatON-Go/rlp"
)

type GovSnapshotDB struct {
	snapdb snapshotdb.DB
}

func NewGovSnapshotDB() GovSnapshotDB {
	return GovSnapshotDB{snapdb: snapshotdb.Instance()}
}
func (self *GovSnapshotDB) reset() {
	self.snapdb.Clear()
	self.snapdb.Close()
	self.snapdb = nil
}

func (self *GovSnapshotDB) get(blockHash common.Hash, key []byte) ([]byte, error) {
	return self.snapdb.Get(blockHash, key)
}

func (self *GovSnapshotDB) put(blockHash common.Hash, key []byte, value interface{}) error {
	bytes, err := rlp.EncodeToBytes(value)
	if err != nil {
		return err
	}
	return self.snapdb.Put(blockHash, key, bytes)
}

func (self *GovSnapshotDB) del(blockHash common.Hash, key []byte) error {
	return self.snapdb.Del(blockHash, key)
}

func (self *GovSnapshotDB) addProposalByKey(blockHash common.Hash, key []byte, proposalId common.Hash) error {
	hashes, err := self.getProposalIDListByKey(blockHash, key)
	if err != nil {
		return err
	}

	hashes = append(hashes, proposalId)

	return self.put(blockHash, key, hashes)
}

func (self *GovSnapshotDB) getVotingIDList(blockHash common.Hash) ([]common.Hash, error) {
	return self.getProposalIDListByKey(blockHash, KeyVotingProposals())
}

func (self *GovSnapshotDB) getPreActiveIDList(blockHash common.Hash) ([]common.Hash, error) {
	return self.getProposalIDListByKey(blockHash, KeyPreActiveProposals())
}

func (self *GovSnapshotDB) getEndIDList(blockHash common.Hash) ([]common.Hash, error) {
	return self.getProposalIDListByKey(blockHash, KeyEndProposals())
}

func (self *GovSnapshotDB) getProposalIDListByKey(blockHash common.Hash, key []byte) ([]common.Hash, error) {
	bytes, err := self.get(blockHash, key)
	if err != nil && err != snapshotdb.ErrNotFound {
		return nil, err
	}
	var idList []common.Hash
	if bytes != nil {
		if err = rlp.DecodeBytes(bytes, &idList); err != nil {
			return nil, err
		}
	}
	return idList, nil
}

func (self *GovSnapshotDB) getAllProposalIDList(blockHash common.Hash) ([]common.Hash, error) {
	var total []common.Hash

	hashes, _ := self.getVotingIDList(blockHash)
	total = append(total, hashes...)

	hashes, _ = self.getPreActiveIDList(blockHash)
	total = append(total, hashes...)

	hashes, _ = self.getEndIDList(blockHash)
	total = append(total, hashes...)
	return total, nil
}

func (self *GovSnapshotDB) addVotedVerifier(blockHash common.Hash, node discover.NodeID, proposalId common.Hash) error {

	nodes, err := self.getVotedVerifierList(blockHash, proposalId)
	if err != nil {
		return err
	}
	nodes = append(nodes, node)

	return self.put(blockHash, keyPrefixVotedVerifiers, nodes)
}

func (self *GovSnapshotDB) getVotedVerifierList(blockHash common.Hash, proposalId common.Hash) ([]discover.NodeID, error) {
	value, err := self.get(blockHash, KeyVotedVerifiers(proposalId))
	if err != nil && err != snapshotdb.ErrNotFound {
		return nil, err
	}
	var nodes []discover.NodeID
	if value != nil {
		if err := rlp.DecodeBytes(value, &nodes); err != nil {
			return nil, err
		}
	}
	return nodes, nil
}

func (self *GovSnapshotDB) addActiveNode(blockHash common.Hash, node discover.NodeID, proposalId common.Hash) error {

	nodes, err := self.getActiveNodeList(blockHash, proposalId)
	if err != nil && err != snapshotdb.ErrNotFound {
		return err
	}
	nodes = append(nodes, node)

	return self.put(blockHash, KeyActiveNodes(proposalId), nodes)
}

func (self *GovSnapshotDB) getActiveNodeList(blockHash common.Hash, proposalId common.Hash) ([]discover.NodeID, error) {
	value, err := self.get(blockHash, KeyActiveNodes(proposalId))
	if err != nil && err != snapshotdb.ErrNotFound {
		return nil, err
	}
	var nodes []discover.NodeID
	if value != nil {
		if err := rlp.DecodeBytes(value, &nodes); err != nil {
			return nil, err
		}
	}
	return nodes, nil
}

func (self *GovSnapshotDB) deleteActiveNodeList(blockHash common.Hash, proposalId common.Hash) error {
	return self.del(blockHash, KeyActiveNodes(proposalId))
}

func (self *GovSnapshotDB) addTotalVerifiers(blockHash common.Hash, proposalId common.Hash, nodes []discover.NodeID) error {
	value, err := self.get(blockHash, KeyAccuVerifier(proposalId))
	if err != nil && err != snapshotdb.ErrNotFound {
		return err
	}
	var verifiers []discover.NodeID

	if value != nil {
		if err := rlp.DecodeBytes(value, &verifiers); err != nil {
			return err
		}
	}
	verifiers = append(verifiers, nodes...)

	return self.put(blockHash, KeyAccuVerifier(proposalId), verifiers)
}

func (self *GovSnapshotDB) getAccuVerifiersLength(blockHash common.Hash, proposalId common.Hash) (uint16, error) {
	value, err := self.get(blockHash, KeyAccuVerifier(proposalId))
	if err != nil && err != snapshotdb.ErrNotFound {
		return 0, err
	}

	if value != nil {
		var verifiers []discover.NodeID
		if err := rlp.DecodeBytes(value, &verifiers); err != nil {
			return 0, err
		} else {
			return uint16(len(verifiers)), nil
		}
	}
	return 0, nil
}
