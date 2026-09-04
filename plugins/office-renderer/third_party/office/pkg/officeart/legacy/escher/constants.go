package escher

const (
	RecordDggContainer    uint16 = 0xF000
	RecordBStoreContainer uint16 = 0xF001
	RecordDgContainer     uint16 = 0xF002
	RecordSpgrContainer   uint16 = 0xF003
	RecordSpContainer     uint16 = 0xF004
	RecordDgg             uint16 = 0xF006
	RecordBSE             uint16 = 0xF007
	RecordDg              uint16 = 0xF008
	RecordFSPGR           uint16 = 0xF009
	RecordFSP             uint16 = 0xF00A
	RecordFOPT            uint16 = 0xF00B
	RecordClientTextbox   uint16 = 0xF00D
	RecordChildAnchor     uint16 = 0xF00F
	RecordClientAnchor    uint16 = 0xF010
	RecordClientData      uint16 = 0xF011
	RecordSecondaryFOPT   uint16 = 0xF121
	RecordTertiaryFOPT    uint16 = 0xF122
	RecordBlipEMF         uint16 = 0xF01A
	RecordBlipWMF         uint16 = 0xF01B
	RecordBlipPICT        uint16 = 0xF01C
	RecordBlipJPEG        uint16 = 0xF01D
	RecordBlipPNG         uint16 = 0xF01E
	RecordBlipDIB         uint16 = 0xF01F
)

const ContainerVersion uint8 = 0x0F
