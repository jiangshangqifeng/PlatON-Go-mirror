package pposm


const (

	/** about candidate pool */
	// immediate elected candidate
	ImmediatePrefix     = "id"
	ImmediateListPrefix = "iL"
	// reserve elected candidate
	ReservePrefix     = "rd"
	ReserveListPrefix = "rL"
	// previous witness
	PreWitnessPrefix     = "Pwn"
	PreWitnessListPrefix = "PwL"
	// witness
	WitnessPrefix     = "wn"
	WitnessListPrefix = "wL"
	// next witness
	NextWitnessPrefix     = "Nwn"
	NextWitnessListPrefix = "NwL"
	// need refund
	DefeatPrefix     = "df"
	DefeatListPrefix = "dL"

	/** about ticket pool */
	// Remaining number key
	SurplusQuantity		= "sq"
	// Expire ticket prefix
	ExpireTicket		= "et"
	AccountExpireTicket	= "ae"
	AccountNormalTicket	= "an"
	// candidate attach
	CandidateAttach	= "ca"

	//ticket id list cache key
	ticketPoolCache = "ticketPoolCache"
)


var (

	/** about candidate pool */
	// immediate elected candidate
	ImmediateBytePrefix     = []byte(ImmediatePrefix)
	ImmediateListBytePrefix = []byte(ImmediateListPrefix)
	// reserve elected candidate
	ReserveBytePrefix     = []byte(ReservePrefix)
	ReserveListBytePrefix = []byte(ReserveListPrefix)
	// previous witness
	PreWitnessBytePrefix     = []byte(PreWitnessPrefix)
	PreWitnessListBytePrefix = []byte(PreWitnessListPrefix)
	// witness
	WitnessBytePrefix     = []byte(WitnessPrefix)
	WitnessListBytePrefix = []byte(WitnessListPrefix)
	// next witness
	NextWitnessBytePrefix     = []byte(NextWitnessPrefix)
	NextWitnessListBytePrefix = []byte(NextWitnessListPrefix)
	// need refund
	DefeatBytePrefix     = []byte(DefeatPrefix)
	DefeatListBytePrefix = []byte(DefeatListPrefix)

	/** about ticket pool */
	// Remaining number key
	SurplusQuantityKey			= []byte(SurplusQuantity)
	// Expire ticket prefix
	ExpireTicketPrefix			= []byte(ExpireTicket)
	AccountExpireTicketPrefix	= []byte(AccountExpireTicket)
	AccountNormalTicketPrefix	= []byte(AccountNormalTicket)
	CandidateAttachPrefix		= []byte(CandidateAttach)

	ticketPoolCacheKey			= []byte(ticketPoolCache)
)