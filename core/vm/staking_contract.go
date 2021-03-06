package vm

import (
	"fmt"
	"math/big"

	"github.com/PlatONnetwork/PlatON-Go/node"

	"github.com/PlatONnetwork/PlatON-Go/x/gov"

	"github.com/PlatONnetwork/PlatON-Go/crypto/bls"

	"github.com/PlatONnetwork/PlatON-Go/params"

	"github.com/PlatONnetwork/PlatON-Go/common"
	"github.com/PlatONnetwork/PlatON-Go/common/vm"
	"github.com/PlatONnetwork/PlatON-Go/core/snapshotdb"
	"github.com/PlatONnetwork/PlatON-Go/log"
	"github.com/PlatONnetwork/PlatON-Go/p2p/discover"
	"github.com/PlatONnetwork/PlatON-Go/x/plugin"
	"github.com/PlatONnetwork/PlatON-Go/x/staking"
	"github.com/PlatONnetwork/PlatON-Go/x/xutil"
)

const (
	TxCreateStaking     = 1000
	TxEditorCandidate   = 1001
	TxIncreaseStaking   = 1002
	TxWithdrewCandidate = 1003
	TxDelegate          = 1004
	TxWithdrewDelegate  = 1005
	QueryVerifierList   = 1100
	QueryValidatorList  = 1101
	QueryCandidateList  = 1102
	QueryRelateList     = 1103
	QueryDelegateInfo   = 1104
	QueryCandidateInfo  = 1105
)

const (
	BLSPUBKEYLEN = 96 //  the bls public key length must be 96 byte
	BLSPROOFLEN  = 64 // the bls proof length must be 64 byte
)

type StakingContract struct {
	Plugin   *plugin.StakingPlugin
	Contract *Contract
	Evm      *EVM
}

func (stkc *StakingContract) RequiredGas(input []byte) uint64 {
	return params.StakingGas
}

func (stkc *StakingContract) Run(input []byte) ([]byte, error) {
	return execPlatonContract(input, stkc.FnSigns())
}

func (stkc *StakingContract) CheckGasPrice(gasPrice *big.Int, fcode uint16) error {
	return nil
}

func (stkc *StakingContract) FnSigns() map[uint16]interface{} {
	return map[uint16]interface{}{
		// Set
		TxCreateStaking:     stkc.createStaking,
		TxEditorCandidate:   stkc.editCandidate,
		TxIncreaseStaking:   stkc.increaseStaking,
		TxWithdrewCandidate: stkc.withdrewStaking,
		TxDelegate:          stkc.delegate,
		TxWithdrewDelegate:  stkc.withdrewDelegate,

		// Get
		QueryVerifierList:  stkc.getVerifierList,
		QueryValidatorList: stkc.getValidatorList,
		QueryCandidateList: stkc.getCandidateList,
		QueryRelateList:    stkc.getRelatedListByDelAddr,
		QueryDelegateInfo:  stkc.getDelegateInfo,
		QueryCandidateInfo: stkc.getCandidateInfo,
	}
}

func (stkc *StakingContract) createStaking(typ uint16, benefitAddress common.Address, nodeId discover.NodeID,
	externalId, nodeName, website, details string, amount *big.Int, programVersion uint32,
	programVersionSign common.VersionSign, blsPubKey bls.PublicKeyHex, blsProof bls.SchnorrProofHex) ([]byte, error) {

	txHash := stkc.Evm.StateDB.TxHash()
	txIndex := stkc.Evm.StateDB.TxIdx()
	blockNumber := stkc.Evm.BlockNumber
	blockHash := stkc.Evm.BlockHash
	from := stkc.Contract.CallerAddress
	state := stkc.Evm.StateDB

	log.Debug("Call createStaking of stakingContract", "txHash", txHash.Hex(),
		"blockNumber", blockNumber.Uint64(), "blockHash", blockHash.Hex(), "typ", typ,
		"benefitAddress", benefitAddress.String(), "nodeId", nodeId.String(), "externalId", externalId,
		"nodeName", nodeName, "website", website, "details", details, "amount", amount,
		"programVersion", programVersion, "programVersionSign", programVersionSign.Hex(),
		"from", from.Hex(), "blsPubKey", blsPubKey, "blsProof", blsProof)

	if !stkc.Contract.UseGas(params.CreateStakeGas) {
		return nil, ErrOutOfGas
	}

	if txHash == common.ZeroHash {
		return nil, nil
	}

	if len(blsPubKey) != BLSPUBKEYLEN {
		return txResultHandler(vm.StakingContractAddr, stkc.Evm, "createStaking",
			fmt.Sprintf("got blsKey length: %d, must be: %d", len(blsPubKey), BLSPUBKEYLEN),
			TxCreateStaking, int(staking.ErrWrongBlsPubKey.Code)), nil
	}

	if len(blsProof) != BLSPROOFLEN {
		return txResultHandler(vm.StakingContractAddr, stkc.Evm, "createStaking",
			fmt.Sprintf("got blsProof length: %d, must be: %d", len(blsProof), BLSPROOFLEN),
			TxCreateStaking, int(staking.ErrWrongBlsPubKeyProof.Code)), nil
	}

	// parse bls publickey
	blsPk, err := blsPubKey.ParseBlsPubKey()
	if nil != err {
		return txResultHandler(vm.StakingContractAddr, stkc.Evm, "createStaking",
			fmt.Sprintf("failed to parse blspubkey: %s", err.Error()),
			TxCreateStaking, int(staking.ErrWrongBlsPubKey.Code)), nil
	}

	// verify bls proof
	if err := verifyBlsProof(blsProof, blsPk); nil != err {
		return txResultHandler(vm.StakingContractAddr, stkc.Evm, "createStaking",
			fmt.Sprintf("failed to verify bls proof: %s", err.Error()),
			TxCreateStaking, int(staking.ErrWrongBlsPubKeyProof.Code)), nil

	}

	// validate programVersion sign
	if !node.GetCryptoHandler().IsSignedByNodeID(programVersion, programVersionSign.Bytes(), nodeId) {
		return txResultHandler(vm.StakingContractAddr, stkc.Evm, "createStaking",
			"call IsSignedByNodeID is failed",
			TxCreateStaking, int(staking.ErrWrongProgramVersionSign.Code)), nil
	}

	if ok, threshold := plugin.CheckStakeThreshold(blockNumber.Uint64(), blockHash, amount); !ok {
		return txResultHandler(vm.StakingContractAddr, stkc.Evm, "createStaking",
			fmt.Sprintf("staking threshold: %d, deposit: %d", threshold, amount),
			TxCreateStaking, int(staking.ErrStakeVonTooLow.Code)), nil
	}

	// check Description length
	desc := &staking.Description{
		NodeName:   nodeName,
		ExternalId: externalId,
		Website:    website,
		Details:    details,
	}
	if err := desc.CheckLength(); nil != err {
		return txResultHandler(vm.StakingContractAddr, stkc.Evm, "createStaking",
			staking.ErrDescriptionLen.Msg+":"+err.Error(),
			TxCreateStaking, int(staking.ErrDescriptionLen.Code)), nil
	}

	// Query current active version
	originVersion := gov.GetVersionForStaking(state)
	currVersion := xutil.CalcVersion(originVersion)
	inputVersion := xutil.CalcVersion(programVersion)

	var isDeclareVersion bool

	// Compare version
	// Just like that:
	// eg: 2.1.x == 2.1.x; 2.1.x > 2.0.x
	if inputVersion < currVersion {
		return txResultHandler(vm.StakingContractAddr, stkc.Evm, "createStaking",
			fmt.Sprintf("input Version: %s, current valid Version: %s",
				xutil.ProgramVersion2Str(programVersion), xutil.ProgramVersion2Str(originVersion)),
			TxCreateStaking, int(staking.ErrProgramVersionTooLow.Code)), nil

	} else if inputVersion > currVersion {
		isDeclareVersion = true
	}

	canAddr, err := xutil.NodeId2Addr(nodeId)
	if nil != err {
		log.Error("Failed to createStaking by parse nodeId", "txHash", txHash,
			"blockNumber", blockNumber, "blockHash", blockHash.Hex(), "nodeId", nodeId.String(), "err", err)
		return nil, err
	}

	canOld, err := stkc.Plugin.GetCandidateInfo(blockHash, canAddr)
	if snapshotdb.NonDbNotFoundErr(err) {
		log.Error("Failed to createStaking by GetCandidateInfo", "txHash", txHash,
			"blockNumber", blockNumber, "err", err)
		return nil, err
	}

	if canOld.IsNotEmpty() {
		return txResultHandler(vm.StakingContractAddr, stkc.Evm, "createStaking",
			"can is not nil",
			TxCreateStaking, int(staking.ErrCanAlreadyExist.Code)), nil
	}

	/**
	init candidate info
	*/
	canBase := &staking.CandidateBase{
		NodeId:          nodeId,
		BlsPubKey:       blsPubKey,
		StakingAddress:  from,
		BenefitAddress:  benefitAddress,
		StakingBlockNum: blockNumber.Uint64(),
		StakingTxIndex:  txIndex,
		ProgramVersion:  currVersion,
		Description:     *desc,
	}

	canMutable := &staking.CandidateMutable{
		Shares:             amount,
		Released:           new(big.Int).SetInt64(0),
		ReleasedHes:        new(big.Int).SetInt64(0),
		RestrictingPlan:    new(big.Int).SetInt64(0),
		RestrictingPlanHes: new(big.Int).SetInt64(0),
	}

	can := &staking.Candidate{}
	can.CandidateBase = canBase
	can.CandidateMutable = canMutable

	err = stkc.Plugin.CreateCandidate(state, blockHash, blockNumber, amount, typ, canAddr, can)

	if nil != err {
		if bizErr, ok := err.(*common.BizError); ok {

			return txResultHandler(vm.StakingContractAddr, stkc.Evm, "createStaking",
				bizErr.Error(), TxCreateStaking, int(bizErr.Code)), nil

		} else {
			log.Error("Failed to createStaking by CreateCandidate", "txHash", txHash,
				"blockNumber", blockNumber, "err", err)
			return nil, err
		}
	}

	// Because we must need to staking before we declare the version information.
	if isDeclareVersion {
		// Declare new Version
		err := gov.DeclareVersion(can.StakingAddress, can.NodeId,
			programVersion, programVersionSign, blockHash, blockNumber.Uint64(), stkc.Plugin, state)
		if nil != err {
			log.Error("Failed to CreateCandidate with govplugin DelareVersion failed",
				"blockNumber", blockNumber.Uint64(), "blockHash", blockHash.Hex(), "err", err)

			if er := stkc.Plugin.RollBackStaking(state, blockHash, blockNumber, canAddr, typ); nil != er {
				log.Error("Failed to createStaking by RollBackStaking", "txHash", txHash,
					"blockNumber", blockNumber, "err", er)
			}

			return txResultHandler(vm.StakingContractAddr, stkc.Evm, "createStaking",
				err.Error(), TxCreateStaking, int(staking.ErrDeclVsFialedCreateCan.Code)), nil

		}
	}

	return txResultHandler(vm.StakingContractAddr, stkc.Evm, "",
		"", TxCreateStaking, int(common.NoErr.Code)), nil
}

func verifyBlsProof(proofHex bls.SchnorrProofHex, pubKey *bls.PublicKey) error {

	proofByte, err := proofHex.MarshalText()
	if nil != err {
		return err
	}

	// proofHex to proof
	proof := new(bls.SchnorrProof)
	if err = proof.UnmarshalText(proofByte); nil != err {
		return err
	}

	// verify proof
	return proof.VerifySchnorrNIZK(*pubKey)
}

func (stkc *StakingContract) editCandidate(benefitAddress common.Address, nodeId discover.NodeID,
	externalId, nodeName, website, details string) ([]byte, error) {

	txHash := stkc.Evm.StateDB.TxHash()
	blockNumber := stkc.Evm.BlockNumber
	blockHash := stkc.Evm.BlockHash
	from := stkc.Contract.CallerAddress

	log.Debug("Call editCandidate of stakingContract", "txHash", txHash.Hex(),
		"blockNumber", blockNumber.Uint64(), "blockHash", blockHash.Hex(),
		"benefitAddress", benefitAddress.String(), "nodeId", nodeId.String(),
		"externalId", externalId, "nodeName", nodeName, "website", website,
		"details", details, "from", from.Hex())

	if !stkc.Contract.UseGas(params.EditCandidatGas) {
		return nil, ErrOutOfGas
	}

	if txHash == common.ZeroHash {
		return nil, nil
	}

	canAddr, err := xutil.NodeId2Addr(nodeId)
	if nil != err {
		log.Error("Failed to editCandidate by parse nodeId", "txHash", txHash,
			"blockNumber", blockNumber, "blockHash", blockHash.Hex(), "nodeId", nodeId.String(), "err", err)
		return nil, err
	}

	canOld, err := stkc.Plugin.GetCandidateInfo(blockHash, canAddr)
	if snapshotdb.NonDbNotFoundErr(err) {
		log.Error("Failed to editCandidate by GetCandidateInfo", "txHash", txHash,
			"blockNumber", blockNumber, "blockHash", blockHash.Hex(), "err", err)
		return nil, err
	}

	if canOld.IsEmpty() {
		return txResultHandler(vm.StakingContractAddr, stkc.Evm, "editCandidate",
			"can is nil", TxEditorCandidate, int(staking.ErrCanNoExist.Code)), nil
	}

	if canOld.IsInvalid() {
		return txResultHandler(vm.StakingContractAddr, stkc.Evm, "editCandidate",
			fmt.Sprintf("can status is: %d", canOld.Status),
			TxEditorCandidate, int(staking.ErrCanStatusInvalid.Code)), nil
	}

	if from != canOld.StakingAddress {
		return txResultHandler(vm.StakingContractAddr, stkc.Evm, "editCandidate",
			fmt.Sprintf("contract sender: %s, can stake addr: %s", from.Hex(), canOld.StakingAddress.Hex()),
			TxEditorCandidate, int(staking.ErrNoSameStakingAddr.Code)), nil
	}

	if canOld.BenefitAddress != vm.RewardManagerPoolAddr {
		canOld.BenefitAddress = benefitAddress
	}

	// check Description length
	desc := &staking.Description{
		NodeName:   nodeName,
		ExternalId: externalId,
		Website:    website,
		Details:    details,
	}
	if err := desc.CheckLength(); nil != err {
		return txResultHandler(vm.StakingContractAddr, stkc.Evm, "editCandidate",
			staking.ErrDescriptionLen.Msg+":"+err.Error(),
			TxEditorCandidate, int(staking.ErrDescriptionLen.Code)), nil
	}

	canOld.Description = *desc

	err = stkc.Plugin.EditCandidate(blockHash, blockNumber, canAddr, canOld)
	if nil != err {
		if bizErr, ok := err.(*common.BizError); ok {
			return txResultHandler(vm.StakingContractAddr, stkc.Evm, "editCandidate",
				bizErr.Error(), TxEditorCandidate, int(bizErr.Code)), nil
		} else {
			log.Error("Failed to editCandidate by EditCandidate", "txHash", txHash,
				"blockNumber", blockNumber, "err", err)
			return nil, err
		}

	}

	return txResultHandler(vm.StakingContractAddr, stkc.Evm, "",
		"", TxEditorCandidate, int(common.NoErr.Code)), nil
}

func (stkc *StakingContract) increaseStaking(nodeId discover.NodeID, typ uint16, amount *big.Int) ([]byte, error) {

	txHash := stkc.Evm.StateDB.TxHash()
	blockNumber := stkc.Evm.BlockNumber
	blockHash := stkc.Evm.BlockHash
	from := stkc.Contract.CallerAddress
	state := stkc.Evm.StateDB

	log.Debug("Call increaseStaking of stakingContract", "txHash", txHash.Hex(),
		"blockNumber", blockNumber.Uint64(), "nodeId", nodeId.String(), "typ", typ,
		"amount", amount, "from", from.Hex())

	if !stkc.Contract.UseGas(params.IncStakeGas) {
		return nil, ErrOutOfGas
	}

	if txHash == common.ZeroHash {
		return nil, nil
	}

	if ok, threshold := plugin.CheckOperatingThreshold(blockNumber.Uint64(), blockHash, amount); !ok {
		return txResultHandler(vm.StakingContractAddr, stkc.Evm, "increaseStaking",
			fmt.Sprintf("increase staking threshold: %d, deposit: %d", threshold, amount),
			TxIncreaseStaking, int(staking.ErrIncreaseStakeVonTooLow.Code)), nil
	}

	canAddr, err := xutil.NodeId2Addr(nodeId)
	if nil != err {
		log.Error("Failed to increaseStaking by parse nodeId", "txHash", txHash,
			"blockNumber", blockNumber, "blockHash", blockHash.Hex(), "nodeId", nodeId.String(), "err", err)
		return nil, err
	}

	canOld, err := stkc.Plugin.GetCandidateInfo(blockHash, canAddr)
	if snapshotdb.NonDbNotFoundErr(err) {
		log.Error("Failed to increaseStaking by GetCandidateInfo", "txHash", txHash,
			"blockNumber", blockNumber, "err", err)
		return nil, err
	}

	if canOld.IsEmpty() {
		return txResultHandler(vm.StakingContractAddr, stkc.Evm, "increaseStaking",
			"can is nil", TxIncreaseStaking, int(staking.ErrCanNoExist.Code)), nil
	}

	if canOld.IsInvalid() {
		return txResultHandler(vm.StakingContractAddr, stkc.Evm, "increaseStaking",
			fmt.Sprintf("can status is: %d", canOld.Status),
			TxIncreaseStaking, int(staking.ErrCanStatusInvalid.Code)), nil
	}

	if from != canOld.StakingAddress {
		return txResultHandler(vm.StakingContractAddr, stkc.Evm, "increaseStaking",
			fmt.Sprintf("contract sender: %s, can stake addr: %s", from.Hex(), canOld.StakingAddress.Hex()),
			TxIncreaseStaking, int(staking.ErrNoSameStakingAddr.Code)), nil
	}

	err = stkc.Plugin.IncreaseStaking(state, blockHash, blockNumber, amount, typ, canAddr, canOld)

	if nil != err {
		if bizErr, ok := err.(*common.BizError); ok {
			return txResultHandler(vm.StakingContractAddr, stkc.Evm, "increaseStaking",
				bizErr.Error(), TxIncreaseStaking, int(bizErr.Code)), nil

		} else {
			log.Error("Failed to increaseStaking by EditCandidate", "txHash", txHash,
				"blockNumber", blockNumber, "err", err)
			return nil, err
		}

	}
	return txResultHandler(vm.StakingContractAddr, stkc.Evm, "",
		"", TxIncreaseStaking, int(common.NoErr.Code)), nil
}

func (stkc *StakingContract) withdrewStaking(nodeId discover.NodeID) ([]byte, error) {

	txHash := stkc.Evm.StateDB.TxHash()
	blockNumber := stkc.Evm.BlockNumber
	blockHash := stkc.Evm.BlockHash
	from := stkc.Contract.CallerAddress
	state := stkc.Evm.StateDB

	log.Debug("Call withdrewStaking of stakingContract", "txHash", txHash.Hex(),
		"blockNumber", blockNumber.Uint64(), "nodeId", nodeId.String(), "from", from.Hex())

	if !stkc.Contract.UseGas(params.WithdrewStakeGas) {
		return nil, ErrOutOfGas
	}

	if txHash == common.ZeroHash {
		return nil, nil
	}

	canAddr, err := xutil.NodeId2Addr(nodeId)
	if nil != err {
		log.Error("Failed to withdrewStaking by parse nodeId", "txHash", txHash,
			"blockNumber", blockNumber, "blockHash", blockHash.Hex(), "nodeId", nodeId.String(), "err", err)
		return nil, err
	}

	canOld, err := stkc.Plugin.GetCandidateInfo(blockHash, canAddr)
	if snapshotdb.NonDbNotFoundErr(err) {
		log.Error("Failed to withdrewStaking by GetCandidateInfo", "txHash", txHash,
			"blockNumber", blockNumber, "blockHash", blockHash.Hex(), "nodeId", nodeId.String(), "err", err)
		return nil, err
	}

	if canOld.IsEmpty() {
		return txResultHandler(vm.StakingContractAddr, stkc.Evm, "withdrewStaking",
			"can is nil", TxWithdrewCandidate, int(staking.ErrCanNoExist.Code)), nil
	}

	if canOld.IsInvalid() {
		return txResultHandler(vm.StakingContractAddr, stkc.Evm, "withdrewStaking",
			fmt.Sprintf("can status is: %d", canOld.Status),
			TxWithdrewCandidate, int(staking.ErrCanStatusInvalid.Code)), nil
	}

	if from != canOld.StakingAddress {
		return txResultHandler(vm.StakingContractAddr, stkc.Evm, "withdrewStaking",
			fmt.Sprintf("contract sender: %s, can stake addr: %s", from.Hex(), canOld.StakingAddress.Hex()),
			TxWithdrewCandidate, int(staking.ErrNoSameStakingAddr.Code)), nil
	}

	err = stkc.Plugin.WithdrewStaking(state, blockHash, blockNumber, canAddr, canOld)
	if nil != err {
		if bizErr, ok := err.(*common.BizError); ok {
			return txResultHandler(vm.StakingContractAddr, stkc.Evm, "withdrewStaking",
				bizErr.Error(), TxWithdrewCandidate, int(bizErr.Code)), nil
		} else {
			log.Error("Failed to withdrewStaking by WithdrewStaking", "txHash", txHash,
				"blockNumber", blockNumber, "err", err)
			return nil, err
		}

	}

	return txResultHandler(vm.StakingContractAddr, stkc.Evm, "",
		"", TxWithdrewCandidate, int(common.NoErr.Code)), nil
}

func (stkc *StakingContract) delegate(typ uint16, nodeId discover.NodeID, amount *big.Int) ([]byte, error) {

	txHash := stkc.Evm.StateDB.TxHash()
	blockNumber := stkc.Evm.BlockNumber
	blockHash := stkc.Evm.BlockHash
	from := stkc.Contract.CallerAddress
	state := stkc.Evm.StateDB

	log.Debug("Call delegate of stakingContract", "txHash", txHash.Hex(),
		"blockNumber", blockNumber.Uint64(), "delAddr", from.Hex(), "typ", typ,
		"nodeId", nodeId.String(), "amount", amount)

	if !stkc.Contract.UseGas(params.DelegateGas) {
		return nil, ErrOutOfGas
	}

	if txHash == common.ZeroHash {
		return nil, nil
	}

	if ok, threshold := plugin.CheckOperatingThreshold(blockNumber.Uint64(), blockHash, amount); !ok {
		return txResultHandler(vm.StakingContractAddr, stkc.Evm, "delegate",
			fmt.Sprintf("delegate threshold: %d, deposit: %d", threshold, amount),
			TxDelegate, int(staking.ErrDelegateVonTooLow.Code)), nil
	}

	// check account
	hasStake, err := stkc.Plugin.HasStake(blockHash, from)
	if nil != err {
		return nil, err
	}

	if hasStake {
		return txResultHandler(vm.StakingContractAddr, stkc.Evm, "delegate",
			fmt.Sprintf("'%s' has staking, so don't allow to delegate", from.Hex()),
			TxDelegate, int(staking.ErrAccountNoAllowToDelegate.Code)), nil
	}

	canAddr, err := xutil.NodeId2Addr(nodeId)
	if nil != err {
		log.Error("Failed to delegate by parse nodeId", "txHash", txHash, "blockNumber",
			blockNumber, "blockHash", blockHash.Hex(), "nodeId", nodeId.String(), "err", err)
		return nil, err
	}

	canMutable, err := stkc.Plugin.GetCanMutable(blockHash, canAddr)
	if snapshotdb.NonDbNotFoundErr(err) {
		log.Error("Failed to delegate by GetCandidateInfo", "txHash", txHash, "blockNumber", blockNumber, "err", err)
		return nil, err
	}

	if canMutable.IsEmpty() {
		return txResultHandler(vm.StakingContractAddr, stkc.Evm, "delegate",
			"can is nil", TxDelegate, int(staking.ErrCanNoExist.Code)), nil
	}

	if canMutable.IsInvalid() {
		return txResultHandler(vm.StakingContractAddr, stkc.Evm, "delegate",
			fmt.Sprintf("can status is: %d", canMutable.Status),
			TxDelegate, int(staking.ErrCanStatusInvalid.Code)), nil
	}

	canBase, err := stkc.Plugin.GetCanBase(blockHash, canAddr)

	// If the candidate???s benefitaAddress is the RewardManagerPoolAddr, no delegation is allowed
	if canBase.BenefitAddress == vm.RewardManagerPoolAddr {
		return txResultHandler(vm.StakingContractAddr, stkc.Evm, "delegate",
			"the can benefitAddr is reward addr",
			TxDelegate, int(staking.ErrCanNoAllowDelegate.Code)), nil
	}

	del, err := stkc.Plugin.GetDelegateInfo(blockHash, from, nodeId, canBase.StakingBlockNum)
	if snapshotdb.NonDbNotFoundErr(err) {
		log.Error("Failed to delegate by GetDelegateInfo", "txHash", txHash, "blockNumber", blockNumber, "err", err)
		return nil, err
	}

	if del.IsEmpty() {
		// build delegate
		del = new(staking.Delegation)
		// Prevent null pointer initialization
		del.Released = new(big.Int).SetInt64(0)
		del.RestrictingPlan = new(big.Int).SetInt64(0)
		del.ReleasedHes = new(big.Int).SetInt64(0)
		del.RestrictingPlanHes = new(big.Int).SetInt64(0)
	}
	can := &staking.Candidate{}
	can.CandidateBase = canBase
	can.CandidateMutable = canMutable

	err = stkc.Plugin.Delegate(state, blockHash, blockNumber, from, del, canAddr, can, typ, amount)
	if nil != err {
		if bizErr, ok := err.(*common.BizError); ok {
			return txResultHandler(vm.StakingContractAddr, stkc.Evm, "delegate",
				bizErr.Error(), TxDelegate, int(bizErr.Code)), nil
		} else {
			log.Error("Failed to delegate by Delegate", "txHash", txHash, "blockNumber", blockNumber, "err", err)
			return nil, err
		}
	}

	return txResultHandler(vm.StakingContractAddr, stkc.Evm, "",
		"", TxDelegate, int(common.NoErr.Code)), nil
}

func (stkc *StakingContract) withdrewDelegate(stakingBlockNum uint64, nodeId discover.NodeID, amount *big.Int) ([]byte, error) {

	txHash := stkc.Evm.StateDB.TxHash()
	blockNumber := stkc.Evm.BlockNumber
	blockHash := stkc.Evm.BlockHash
	from := stkc.Contract.CallerAddress
	state := stkc.Evm.StateDB

	log.Debug("Call withdrewDelegate of stakingContract", "txHash", txHash.Hex(),
		"blockNumber", blockNumber.Uint64(), "delAddr", from.Hex(), "nodeId", nodeId.String(),
		"stakingNum", stakingBlockNum, "amount", amount)

	if !stkc.Contract.UseGas(params.WithdrewDelegateGas) {
		return nil, ErrOutOfGas
	}

	if txHash == common.ZeroHash {
		return nil, nil
	}

	if ok, threshold := plugin.CheckOperatingThreshold(blockNumber.Uint64(), blockHash, amount); !ok {

		return txResultHandler(vm.StakingContractAddr, stkc.Evm, "withdrewDelegate",
			fmt.Sprintf("withdrewDelegate threshold: %d, deposit: %d", threshold, amount),
			TxWithdrewDelegate, int(staking.ErrWithdrewDelegateVonTooLow.Code)), nil
	}

	del, err := stkc.Plugin.GetDelegateInfo(blockHash, from, nodeId, stakingBlockNum)
	if snapshotdb.NonDbNotFoundErr(err) {
		log.Error("Failed to withdrewDelegate by GetDelegateInfo",
			"txHash", txHash.Hex(), "blockNumber", blockNumber, "err", err)
		return nil, err
	}

	if del.IsEmpty() {

		return txResultHandler(vm.StakingContractAddr, stkc.Evm, "withdrewDelegate",
			"del is nil", TxWithdrewDelegate, int(staking.ErrDelegateNoExist.Code)), nil
	}

	err = stkc.Plugin.WithdrewDelegate(state, blockHash, blockNumber, amount, from, nodeId, stakingBlockNum, del)
	if nil != err {
		if bizErr, ok := err.(*common.BizError); ok {

			return txResultHandler(vm.StakingContractAddr, stkc.Evm, "withdrewDelegate",
				bizErr.Error(), TxWithdrewDelegate, int(bizErr.Code)), nil

		} else {
			log.Error("Failed to withdrewDelegate by WithdrewDelegate", "txHash", txHash, "blockNumber", blockNumber, "err", err)
			return nil, err
		}
	}

	return txResultHandler(vm.StakingContractAddr, stkc.Evm, "",
		"", TxWithdrewDelegate, int(common.NoErr.Code)), nil
}

func (stkc *StakingContract) getVerifierList() ([]byte, error) {

	blockNumber := stkc.Evm.BlockNumber
	blockHash := stkc.Evm.BlockHash

	arr, err := stkc.Plugin.GetVerifierList(blockHash, blockNumber.Uint64(), plugin.QueryStartIrr)

	if snapshotdb.NonDbNotFoundErr(err) {
		return callResultHandler(stkc.Evm, "getVerifierList",
			arr, staking.ErrGetVerifierList.Wrap(err.Error())), nil
	}

	if snapshotdb.IsDbNotFoundErr(err) || arr.IsEmpty() {
		return callResultHandler(stkc.Evm, "getVerifierList",
			arr, staking.ErrGetVerifierList.Wrap("VerifierList info is not found")), nil
	}

	return callResultHandler(stkc.Evm, "getVerifierList",
		arr, nil), nil
}

func (stkc *StakingContract) getValidatorList() ([]byte, error) {

	blockNumber := stkc.Evm.BlockNumber
	blockHash := stkc.Evm.BlockHash

	arr, err := stkc.Plugin.GetValidatorList(blockHash, blockNumber.Uint64(), plugin.CurrentRound, plugin.QueryStartIrr)
	if snapshotdb.NonDbNotFoundErr(err) {

		return callResultHandler(stkc.Evm, "getValidatorList",
			arr, staking.ErrGetValidatorList.Wrap(err.Error())), nil
	}

	if snapshotdb.IsDbNotFoundErr(err) || arr.IsEmpty() {
		return callResultHandler(stkc.Evm, "getValidatorList",
			arr, staking.ErrGetValidatorList.Wrap("ValidatorList info is not found")), nil
	}

	return callResultHandler(stkc.Evm, "getValidatorList",
		arr, nil), nil
}

func (stkc *StakingContract) getCandidateList() ([]byte, error) {

	blockNumber := stkc.Evm.BlockNumber
	blockHash := stkc.Evm.BlockHash

	arr, err := stkc.Plugin.GetCandidateList(blockHash, blockNumber.Uint64())
	if snapshotdb.NonDbNotFoundErr(err) {
		return callResultHandler(stkc.Evm, "getCandidateList",
			arr, staking.ErrGetCandidateList.Wrap(err.Error())), nil
	}

	if snapshotdb.IsDbNotFoundErr(err) || arr.IsEmpty() {
		return callResultHandler(stkc.Evm, "getCandidateList",
			arr, staking.ErrGetCandidateList.Wrap("CandidateList info is not found")), nil
	}

	return callResultHandler(stkc.Evm, "getCandidateList",
		arr, nil), nil
}

func (stkc *StakingContract) getRelatedListByDelAddr(addr common.Address) ([]byte, error) {

	blockHash := stkc.Evm.BlockHash
	arr, err := stkc.Plugin.GetRelatedListByDelAddr(blockHash, addr)
	if snapshotdb.NonDbNotFoundErr(err) {
		return callResultHandler(stkc.Evm, fmt.Sprintf("getRelatedListByDelAddr, delAddr: %s", addr),
			arr, staking.ErrGetDelegateRelated.Wrap(err.Error())), nil
	}

	if snapshotdb.IsDbNotFoundErr(err) || arr.IsEmpty() {
		return callResultHandler(stkc.Evm, fmt.Sprintf("getRelatedListByDelAddr, delAddr: %s", addr),
			arr, staking.ErrGetDelegateRelated.Wrap("RelatedList info is not found")), nil
	}

	return callResultHandler(stkc.Evm, fmt.Sprintf("getRelatedListByDelAddr, delAddr: %s", addr),
		arr, nil), nil
}

func (stkc *StakingContract) getDelegateInfo(stakingBlockNum uint64, delAddr common.Address,
	nodeId discover.NodeID) ([]byte, error) {

	blockNumber := stkc.Evm.BlockNumber
	blockHash := stkc.Evm.BlockHash

	del, err := stkc.Plugin.GetDelegateExCompactInfo(blockHash, blockNumber.Uint64(), delAddr, nodeId, stakingBlockNum)
	if snapshotdb.NonDbNotFoundErr(err) {
		return callResultHandler(stkc.Evm, fmt.Sprintf("getDelegateInfo, delAddr: %s, nodeId: %s, stakingBlockNumber: %d",
			delAddr, nodeId, stakingBlockNum),
			del, staking.ErrQueryDelegateInfo.Wrap(err.Error())), nil
	}

	if snapshotdb.IsDbNotFoundErr(err) || del.IsEmpty() {
		return callResultHandler(stkc.Evm, fmt.Sprintf("getDelegateInfo, delAddr: %s, nodeId: %s, stakingBlockNumber: %d",
			delAddr, nodeId, stakingBlockNum),
			del, staking.ErrQueryDelegateInfo.Wrap("Delegate info is not found")), nil
	}

	return callResultHandler(stkc.Evm, fmt.Sprintf("getDelegateInfo, delAddr: %s, nodeId: %s, stakingBlockNumber: %d",
		delAddr, nodeId, stakingBlockNum),
		del, nil), nil
}

func (stkc *StakingContract) getCandidateInfo(nodeId discover.NodeID) ([]byte, error) {

	blockNumber := stkc.Evm.BlockNumber
	blockHash := stkc.Evm.BlockHash

	canAddr, err := xutil.NodeId2Addr(nodeId)
	if nil != err {
		return callResultHandler(stkc.Evm, fmt.Sprintf("getCandidateInfo, nodeId: %s",
			nodeId), nil, staking.ErrQueryCandidateInfo.Wrap(err.Error())), nil
	}
	can, err := stkc.Plugin.GetCandidateCompactInfo(blockHash, blockNumber.Uint64(), canAddr)
	if snapshotdb.NonDbNotFoundErr(err) {
		return callResultHandler(stkc.Evm, fmt.Sprintf("getCandidateInfo, nodeId: %s",
			nodeId), can, staking.ErrQueryCandidateInfo.Wrap(err.Error())), nil
	}

	if snapshotdb.IsDbNotFoundErr(err) || can.IsEmpty() {
		return callResultHandler(stkc.Evm, fmt.Sprintf("getCandidateInfo, nodeId: %s",
			nodeId), can, staking.ErrQueryCandidateInfo.Wrap("Candidate info is not found")), nil
	}

	return callResultHandler(stkc.Evm, fmt.Sprintf("getCandidateInfo, nodeId: %s",
		nodeId), can, nil), nil
}
