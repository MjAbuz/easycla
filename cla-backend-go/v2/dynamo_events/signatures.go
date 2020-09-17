// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package dynamo_events

import (
	"errors"
	"fmt"

	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/aws/aws-lambda-go/events"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
)

// constants
const (
	CLASignatureType  = "cla"
	CCLASignatureType = "ccla"

	ICLASignatureType = "icla"
	ECLASignatureType = "ecla"
)

// Signature database model
type Signature struct {
	SignatureID                   string   `json:"signature_id"`
	DateCreated                   string   `json:"date_created"`
	DateModified                  string   `json:"date_modified"`
	SignatureApproved             bool     `json:"signature_approved"`
	SignatureSigned               bool     `json:"signature_signed"`
	SignatureDocumentMajorVersion string   `json:"signature_document_major_version"`
	SignatureDocumentMinorVersion string   `json:"signature_document_minor_version"`
	SignatureReferenceID          string   `json:"signature_reference_id"`
	SignatureReferenceName        string   `json:"signature_reference_name"`
	SignatureReferenceNameLower   string   `json:"signature_reference_name_lower"`
	SignatureProjectID            string   `json:"signature_project_id"`
	SignatureReferenceType        string   `json:"signature_reference_type"`
	SignatureType                 string   `json:"signature_type"`
	SignatureUserCompanyID        string   `json:"signature_user_ccla_company_id"`
	EmailWhitelist                []string `json:"email_whitelist"`
	DomainWhitelist               []string `json:"domain_whitelist"`
	GitHubWhitelist               []string `json:"github_whitelist"`
	GitHubOrgWhitelist            []string `json:"github_org_whitelist"`
	SignatureACL                  []string `json:"signature_acl"`
	SigtypeSignedApprovedID       string   `json:"sigtype_signed_approved_id"`
	UserGithubUsername            string   `json:"user_github_username"`
	UserLFUsername                string   `json:"user_lf_username"`
	UserName                      string   `json:"user_name"`
	UserEmail                     string   `json:"user_email"`
	SignedOn                      string   `json:"signed_on"`
}

// should be called when we modify signature
func (s *service) SignatureSignedEvent(event events.DynamoDBEventRecord) error {
	f := logrus.Fields{
		"functionName": "SignatureSignedEvent",
	}

	// Decode the pre-update and post-update signature record details
	var newSignature, oldSignature Signature
	err := unmarshalStreamImage(event.Change.OldImage, &oldSignature)
	if err != nil {
		log.WithFields(f).Warnf("problem decoding pre-update signature, error: %+v", err)
		return err
	}
	err = unmarshalStreamImage(event.Change.NewImage, &newSignature)
	if err != nil {
		log.WithFields(f).Warnf("problem decoding post-update signature, error: %+v", err)
		return err
	}

	// Add some details for our logger
	f["id"] = newSignature.SignatureID
	f["type"] = newSignature.SignatureType
	f["referenceID"] = newSignature.SignatureReferenceID
	f["referenceName"] = newSignature.SignatureReferenceName
	f["referenceType"] = newSignature.SignatureReferenceType
	f["projectID"] = newSignature.SignatureProjectID
	f["approved"] = newSignature.SignatureApproved
	f["signed"] = newSignature.SignatureSigned

	// check if signature signed event is received
	if !oldSignature.SignatureSigned && newSignature.SignatureSigned {
		// Update the signed on date
		err = s.signatureRepo.AddSignedOn(newSignature.SignatureID)
		if err != nil {
			log.WithFields(f).Warnf("failed to add signed_on date/time to signature, error: %+v", err)
		}

		// If a CCLA signature...
		if newSignature.SignatureType == CCLASignatureType {

			if len(newSignature.SignatureACL) == 0 {
				log.WithFields(f).Warn("initial cla manager details not found")
				return errors.New("initial cla manager details not found")
			}

			log.WithFields(f).Debugf("loading company from signature by companyID: %s...", newSignature.SignatureReferenceID)
			companyModel, err := s.companyRepo.GetCompany(newSignature.SignatureReferenceID)
			if err != nil {
				log.WithFields(f).Warnf("failed to lookup company from signature by companyID: %s, error: %+v",
					newSignature.SignatureReferenceID, err)
				return err
			}
			if companyModel == nil {
				msg := fmt.Sprintf("failed to lookup company from signature by companyID: %s, not found",
					newSignature.SignatureReferenceID)
				log.WithFields(f).Warn(msg)
				return errors.New(msg)
			}

			// We should have the company SFID...
			if companyModel.CompanyExternalID == "" {
				msg := fmt.Sprintf("company %s (%s) does not have a SF Organization ID - unable to update permissions",
					companyModel.CompanyName, companyModel.CompanyID)
				log.WithFields(f).Warn(msg)
				return errors.New(msg)
			}

			var eg errgroup.Group

			// We have a separate routine to help assign the CLA Manager Role - it's a bit wasteful as it
			// loads the signature and other details again.
			// Kick off a go routine to set the cla manager role
			eg.Go(func() error {
				// Set the CLA manager permissions
				log.WithFields(f).Debug("assigning the initial CLA manager")
				err = s.SetInitialCLAManagerACSPermissions(newSignature.SignatureID)
				if err != nil {
					log.WithFields(f).Warnf("failed to set initial cla manager, error: %+v", err)
					return err
				}
				return nil
			})

			// Load the list of SF projects associated with this CLA Group
			log.WithFields(f).Debugf("querying SF projects for CLA Group: %s", newSignature.SignatureProjectID)
			projectCLAGroups, err := s.projectsClaGroupRepo.GetProjectsIdsForClaGroup(newSignature.SignatureProjectID)

			// Kick off a set of go routines to adjust the roles
			for _, projectCLAGroup := range projectCLAGroups {
				eg.Go(func() error {
					// Remove any roles that were previously assigned for cla-manager-designee
					log.WithFields(f).Debugf("removing %s role for project: %s (%s)", utils.CLADesigneeRole,
						projectCLAGroup.ProjectName, projectCLAGroup.ProjectSFID)
					err = s.removeCLAPermissionsByProjectOrganizationRole(projectCLAGroup.ProjectSFID, companyModel.CompanyExternalID, utils.CLADesigneeRole)
					if err != nil {
						log.WithFields(f).Warnf("failed to remove %s roles for project: %s (%s), error: %+v",
							utils.CLADesigneeRole, projectCLAGroup.ProjectName, projectCLAGroup.ProjectSFID, err)
						return err
					}

					return nil
				})
			}

			// Wait for the go routines to finish
			log.WithFields(f).Debug("waiting for role assignment and cleanup...")
			if loadErr := eg.Wait(); loadErr != nil {
				return loadErr
			}
			return nil
		}
	}

	return nil
}

// SignatureAdded function should be called when new icla, ecla signature added
func (s *service) SignatureAddSigTypeSignedApprovedID(event events.DynamoDBEventRecord) error {
	var newSig Signature
	var sigType string
	var id string
	err := unmarshalStreamImage(event.Change.NewImage, &newSig)
	if err != nil {
		return err
	}
	switch {
	case newSig.SignatureType == CCLASignatureType:
		sigType = CCLASignatureType
		id = newSig.SignatureReferenceID
	case newSig.SignatureType == CLASignatureType && newSig.SignatureUserCompanyID == "":
		sigType = ICLASignatureType
		id = newSig.SignatureReferenceID
	case newSig.SignatureType == CLASignatureType && newSig.SignatureUserCompanyID != "":
		sigType = ECLASignatureType
		id = newSig.SignatureUserCompanyID
	default:
		log.Warnf("setting sigtype_signed_approved_id for signature: %s failed", newSig.SignatureID)
		return errors.New("invalid signature in SignatureAddSigTypeSignedApprovedID")
	}
	val := fmt.Sprintf("%s#%v#%v#%s", sigType, newSig.SignatureSigned, newSig.SignatureApproved, id)
	if newSig.SigtypeSignedApprovedID == val {
		return nil
	}
	log.Debugf("setting sigtype_signed_approved_id for signature: %s", newSig.SignatureID)
	err = s.signatureRepo.AddSigTypeSignedApprovedID(newSig.SignatureID, val)
	if err != nil {
		return err
	}
	return nil
}

func (s *service) SignatureAddUsersDetails(event events.DynamoDBEventRecord) error {
	var newSig Signature
	err := unmarshalStreamImage(event.Change.NewImage, &newSig)
	if err != nil {
		return err
	}
	if newSig.SignatureReferenceType == "user" && newSig.UserLFUsername == "" && newSig.UserGithubUsername == "" {
		log.Debugf("adding users details in signature: %s", newSig.SignatureID)
		err = s.signatureRepo.AddUsersDetails(newSig.SignatureID, newSig.SignatureReferenceID)
		if err != nil {
			log.Debugf("adding users details in signature: %s failed. error = %s", newSig.SignatureID, err.Error())
		}
	}
	return nil
}
