package core

import "encoding/json"

type ArtistInfoDTO struct {
	LinkURL     string `json:"link"`
	BioText     string `json:"bio"`
	ServiceName string `json:"bio_service"`
}

func (d ArtistInfoDTO) Link() string       { return d.LinkURL }
func (d ArtistInfoDTO) Bio() string        { return d.BioText }
func (d ArtistInfoDTO) BioService() string { return d.ServiceName }

func artistInfoToBytes(info ArtistInfoer) ([]byte, error) {
	if info == nil {
		return nil, nil
	}
	dto := ArtistInfoDTO{
		LinkURL:     info.Link(),
		BioText:     info.Bio(),
		ServiceName: info.BioService(),
	}
	return json.Marshal(dto)
}

func bytesToArtistInfo(data []byte) (ArtistInfoer, error) {
	var dto ArtistInfoDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		return nil, err
	}
	return dto, nil
}
