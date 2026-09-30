package validate

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type IDType string

type Gender string

type Citizenship string

const (
	IDNIN IDType = "NG_NIN"
	IDBVN IDType = "NG_BVN"
	IDGHA IDType = "GH_CARD"
	IDKID IDType = "KE_ID"
	IDKRA IDType = "KE_KRA"
	IDSA  IDType = "ZA_ID"
	IDFAY IDType = "ET_FAYDA"

	GenderMale   Gender = "M"
	GenderFemale Gender = "F"
	GenderOther  Gender = "U"

	StatusCitizen  Citizenship = "CITIZEN"
	StatusResident Citizenship = "RESIDENT"
)

var (
	errInvalidLength  = errors.New("identity: invalid character length")
	errInvalidFormat  = errors.New("identity: invalid format or prefix")
	errFailedChecksum = errors.New("identity: mathematical checksum failure")

	reNigeriaID     = regexp.MustCompile(`^\d{11}$`)
	reGhanaCard     = regexp.MustCompile(`^GHA-\d{9}-\d{1}$`)
	reKenyaKRA      = regexp.MustCompile(`^[A-Z]\d{9}[A-Z]$`)
	reSouthAfricaID = regexp.MustCompile(`^\d{13}$`)
)

type Result struct {
	IsValid bool
	IDType  IDType
	Cleaned string
	Error   error
}

type ExtractedInfo struct {
	DOB         time.Time
	Gender      Gender
	Citizenship Citizenship
}

func Check(id string, t IDType) Result {
	cleaned := strings.ToUpper(strings.TrimSpace(id))
	res := Result{IDType: t, Cleaned: cleaned}

	switch t {
	case IDNIN, IDBVN:
		if !reNigeriaID.MatchString(cleaned) {
			res.Error = errInvalidLength
			return res
		}
	case IDGHA:
		if !reGhanaCard.MatchString(cleaned) {
			res.Error = errInvalidFormat
			return res
		}
	case IDSA:
		if !reSouthAfricaID.MatchString(cleaned) {
			res.Error = errInvalidLength
			return res
		}
		if !luhnCheck(cleaned) {
			res.Error = errFailedChecksum
			return res
		}
	case IDKRA:
		if !reKenyaKRA.MatchString(cleaned) {
			res.Error = errInvalidFormat
			return res
		}
	case IDKID:
		if len(cleaned) < 7 || len(cleaned) > 12 || !regexp.MustCompile(`^[A-Z0-9]+$`).MatchString(cleaned) {
			res.Error = errInvalidFormat
			return res
		}
	case IDFAY:
		if len(cleaned) == 0 {
			res.Error = errInvalidFormat
			return res
		}
	default:
		res.Error = errInvalidFormat
		return res
	}

	res.IsValid = true
	return res
}

func luhnCheck(id string) bool {
	var sum int
	alt := false
	for i := len(id) - 1; i >= 0; i-- {
		digit := int(id[i] - '0')
		if alt {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
		alt = !alt
	}
	return sum%10 == 0
}

func Mask(id string, t IDType) string {
	cleaned := strings.TrimSpace(id)
	if cleaned == "" {
		return ""
	}

	switch t {
	case IDNIN, IDBVN:
		return maskDigits(cleaned, 3)
	case IDGHA:
		if len(cleaned) >= 4 {
			return cleaned[:3] + "-*****-" + cleaned[len(cleaned)-1:]
		}
	case IDKRA:
		if len(cleaned) >= 4 {
			return cleaned[:1] + "********" + cleaned[len(cleaned)-1:]
		}
	case IDSA:
		if len(cleaned) == 13 {
			return "*******" + cleaned[7:11] + "*"
		}
	}
	return maskDigits(cleaned, 4)
}

func maskDigits(s string, keep int) string {
	if len(s) <= keep {
		return strings.Repeat("*", len(s))
	}
	return strings.Repeat("*", len(s)-keep) + s[len(s)-keep:]
}

func (r Result) Enrich() (*ExtractedInfo, error) {
	if !r.IsValid {
		return nil, r.Error
	}
	switch r.IDType {
	case IDSA:
		return ExtractZA(r.Cleaned)
	default:
		return nil, nil
	}
}

func ExtractZA(id string) (*ExtractedInfo, error) {
	if len(id) != 13 {
		return nil, errInvalidLength
	}
	if !reSouthAfricaID.MatchString(id) {
		return nil, errInvalidFormat
	}

	yearPart, err := strconv.Atoi(id[0:2])
	if err != nil {
		return nil, errInvalidFormat
	}
	monthPart, err := strconv.Atoi(id[2:4])
	if err != nil {
		return nil, errInvalidFormat
	}
	dayPart, err := strconv.Atoi(id[4:6])
	if err != nil {
		return nil, errInvalidFormat
	}
	if monthPart < 1 || monthPart > 12 || dayPart < 1 || dayPart > 31 {
		return nil, errInvalidFormat
	}

	fullYear := fullYearFromTwoDigits(yearPart)
	dob := time.Date(fullYear, time.Month(monthPart), dayPart, 0, 0, 0, 0, time.UTC)

	genderDigits, err := strconv.Atoi(id[6:10])
	if err != nil {
		return nil, errInvalidFormat
	}
	gender := GenderFemale
	if genderDigits >= 5000 {
		gender = GenderMale
	}

	citizenship := StatusCitizen
	if id[10] == '1' {
		citizenship = StatusResident
	}

	return &ExtractedInfo{
		DOB:         dob,
		Gender:      gender,
		Citizenship: citizenship,
	}, nil
}

func fullYearFromTwoDigits(yearPart int) int {
	currentYear := time.Now().Year() % 100
	if yearPart > currentYear {
		return 1900 + yearPart
	}
	return 2000 + yearPart
}

func FingerprintID(id string) string {
	h := sha256.New()
	h.Write([]byte(strings.TrimSpace(strings.ToUpper(id))))
	return fmt.Sprintf("fp_%x", h.Sum(nil))
}
