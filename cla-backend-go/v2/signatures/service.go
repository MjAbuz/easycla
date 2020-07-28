package signatures

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"

	"github.com/jinzhu/copier"

	"github.com/communitybridge/easycla/cla-backend-go/company"
	v1Models "github.com/communitybridge/easycla/cla-backend-go/gen/models"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/models"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/communitybridge/easycla/cla-backend-go/project"
	"github.com/communitybridge/easycla/cla-backend-go/signatures"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
)

// constants
const (
	// used when we want to query all data from dependent service.
	HugePageSize      = int64(10000)
	CclaSignatureType = "ccla"
	ClaSignatureType  = "cla"
)

type service struct {
	v1ProjectService      project.Service
	v1CompanyService      company.IService
	v1SignatureService    signatures.SignatureService
	projectsClaGroupsRepo projects_cla_groups.Repository
}

// Service contains method of v2 signature service
type Service interface {
	GetProjectCompanySignatures(companySFID string, projectSFID string) (*models.Signatures, error)
	GetProjectIclaSignaturesCsv(claGroupID string) ([]byte, error)
	GetProjectIclaSignatures(claGroupID string, searchTerm *string) (*models.IclaSignatures, error)
	GetClaGroupCorporateContributors(claGroupID string, companySFID *string, searchTerm *string) (*models.CorporateContributorList, error)
	GetSignedDocument(signatureID string) (*models.SignedDocument, error)
	GetSignedIclaZipPdf(claGroupID string) (*models.URLObject, error)
	GetSignedCclaZipPdf(claGroupID string) (*models.URLObject, error)
}

// NewService creates instance of v2 signature service
func NewService(v1ProjectService project.Service,
	v1CompanyService company.IService,
	v1SignatureService signatures.SignatureService,
	pcgRepo projects_cla_groups.Repository) *service {
	return &service{
		v1ProjectService:      v1ProjectService,
		v1CompanyService:      v1CompanyService,
		v1SignatureService:    v1SignatureService,
		projectsClaGroupsRepo: pcgRepo,
	}
}

func (s *service) GetProjectCompanySignatures(companySFID string, projectSFID string) (*models.Signatures, error) {
	companyModel, err := s.v1CompanyService.GetCompanyByExternalID(companySFID)
	if err != nil {
		return nil, err
	}
	pm, err := s.projectsClaGroupsRepo.GetClaGroupIDForProject(projectSFID)
	if err != nil {
		return nil, err
	}
	signed := true
	approved := true
	sig, err := s.v1SignatureService.GetProjectCompanySignature(companyModel.CompanyID, pm.ClaGroupID, &signed, &approved, nil, aws.Int64(HugePageSize))
	if err != nil {
		return nil, err
	}
	resp := &v1Models.Signatures{
		Signatures: make([]*v1Models.Signature, 0),
	}
	if sig != nil {
		resp.ResultCount = 1
		resp.Signatures = append(resp.Signatures, sig)
	}
	return v2SignaturesReplaceCompanyID(resp, companyModel.CompanyID, companySFID)
}

func iclaSigCsvLine(sig *v1Models.IclaSignature) string {
	var dateTime string
	t, err := utils.ParseDateTime(sig.SignedOn)
	if err != nil {
		log.WithFields(logrus.Fields{"signature_id": sig.SignatureID, "signature_created": sig.SignedOn}).
			Error("invalid time format present for signatures")
	} else {
		dateTime = t.Format("Jan 2,2006")
	}
	return fmt.Sprintf("\n%s,%s,%s,%s,\"%s\"", sig.GithubUsername, sig.LfUsername, sig.UserName, sig.UserEmail, dateTime)
}

func (s service) GetProjectIclaSignaturesCsv(claGroupID string) ([]byte, error) {
	var b bytes.Buffer
	result, err := s.v1SignatureService.GetClaGroupICLASignatures(claGroupID, nil)
	if err != nil {
		return nil, err
	}
	b.WriteString(`Github ID,LF_ID,Name,Email,Date Signed`)
	for _, sig := range result.List {
		b.WriteString(iclaSigCsvLine(sig))
	}
	return b.Bytes(), nil
}

func (s service) GetProjectIclaSignatures(claGroupID string, searchTerm *string) (*models.IclaSignatures, error) {
	var out models.IclaSignatures
	result, err := s.v1SignatureService.GetClaGroupICLASignatures(claGroupID, searchTerm)
	if err != nil {
		return nil, err
	}
	err = copier.Copy(&out, result)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (s service) GetSignedDocument(signatureID string) (*models.SignedDocument, error) {
	sig, err := s.v1SignatureService.GetSignature(signatureID)
	if err != nil {
		return nil, err
	}
	if sig.SignatureType == ClaSignatureType && sig.CompanyName != "" {
		return nil, errors.New("bad request. employee signature does not have signed document")
	}
	var url string
	switch sig.SignatureType {
	case ClaSignatureType:
		url = utils.SignedCLAFilename(sig.ProjectID, "icla", sig.SignatureReferenceID, sig.SignatureID)
	case CclaSignatureType:
		url = utils.SignedCLAFilename(sig.ProjectID, "ccla", sig.SignatureReferenceID, sig.SignatureID)
	}
	signedURL, err := utils.GetDownloadLink(url)
	if err != nil {
		return nil, err
	}
	return &models.SignedDocument{
		SignatureID:  signatureID,
		SignedClaURL: signedURL,
	}, nil
}

func (s service) GetSignedCclaZipPdf(claGroupID string) (*models.URLObject, error) {
	url := utils.SignedClaGroupZipFilename(claGroupID, CCLA)
	signedURL, err := utils.GetDownloadLink(url)
	if err != nil {
		return nil, err
	}
	return &models.URLObject{
		URL: signedURL,
	}, nil
}
func (s service) GetSignedIclaZipPdf(claGroupID string) (*models.URLObject, error) {
	url := utils.SignedClaGroupZipFilename(claGroupID, ICLA)
	signedURL, err := utils.GetDownloadLink(url)
	if err != nil {
		return nil, err
	}
	return &models.URLObject{
		URL: signedURL,
	}, nil
}

func (s service) GetClaGroupCorporateContributors(claGroupID string, companySFID *string, searchTerm *string) (*models.CorporateContributorList, error) {
	var companyID *string
	if companySFID != nil {
		companyModel, err := s.v1CompanyService.GetCompanyByExternalID(*companySFID)
		if err != nil {
			return nil, err
		}
		companyID = &companyModel.CompanyID
	}
	result, err := s.v1SignatureService.GetClaGroupCorporateContributors(claGroupID, companyID, searchTerm)
	if err != nil {
		return nil, err
	}
	var resp models.CorporateContributorList
	err = copier.Copy(&resp, result)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
