// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_manager

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/go-openapi/strfmt"

	"github.com/sirupsen/logrus"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/communitybridge/easycla/cla-backend-go/events"
	"github.com/communitybridge/easycla/cla-backend-go/utils"

	"github.com/communitybridge/easycla/cla-backend-go/company"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/models"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations/cla_manager"
	"github.com/communitybridge/easycla/cla-backend-go/project"
	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"
	"github.com/communitybridge/easycla/cla-backend-go/repositories"
	"github.com/communitybridge/easycla/cla-backend-go/v2/organization-service/client/organizations"

	v1ClaManager "github.com/communitybridge/easycla/cla-backend-go/cla_manager"
	v1Models "github.com/communitybridge/easycla/cla-backend-go/gen/models"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	v1User "github.com/communitybridge/easycla/cla-backend-go/user"
	easyCLAUser "github.com/communitybridge/easycla/cla-backend-go/users"
	v2AcsService "github.com/communitybridge/easycla/cla-backend-go/v2/acs-service"
	v2Company "github.com/communitybridge/easycla/cla-backend-go/v2/company"
	v2OrgService "github.com/communitybridge/easycla/cla-backend-go/v2/organization-service"
	v2ProjectService "github.com/communitybridge/easycla/cla-backend-go/v2/project-service"
	v2UserService "github.com/communitybridge/easycla/cla-backend-go/v2/user-service"
)

var (
	//ErrSalesForceProjectNotFound returned error if salesForce Project not found
	ErrSalesForceProjectNotFound = errors.New("salesforce project not found")
	//ErrCLACompanyNotFound returned if EasyCLA company not found
	ErrCLACompanyNotFound = errors.New("company not found")
	//ErrGitHubRepoNotFound returned if GH Repos is not found
	ErrGitHubRepoNotFound = errors.New("github repo not found")
	//ErrCLAUserNotFound returned if EasyCLA User is not found
	ErrCLAUserNotFound = errors.New("cla user not found")
	//ErrCLAManagersNotFound when cla managers arent found for given  project and company
	ErrCLAManagersNotFound = errors.New("cla managers not found")
	//ErrLFXUserNotFound when user-service fails to find user
	ErrLFXUserNotFound = errors.New("lfx user not found")
	//ErrNoLFID thrown when users dont have an LFID
	ErrNoLFID = errors.New("user has no LF Login")
	//ErrNotInOrg when user is not in organization
	ErrNotInOrg = errors.New("user not in organization")
	//ErrNoOrgAdmins when No admins found for organization
	ErrNoOrgAdmins = errors.New("no admins in company")
	//ErrRoleScopeConflict thrown if user already has role scope
	ErrRoleScopeConflict = errors.New("user is already cla-manager")
	//ErrCLAManagerDesigneeConflict when user is already assigned cla-manager-designee role
	ErrCLAManagerDesigneeConflict = errors.New("user already assigned cla-manager")
	//ErrScopeNotFound returns error when getting scopeID
	ErrScopeNotFound = errors.New("scope not found")
	//ErrProjectSigned returns error if project already signed
	ErrProjectSigned = errors.New("project already signed")
)

const (
	// NoAccount represents user with no company
	NoAccount = "Individual - No Account"
)

type service struct {
	companyService      company.IService
	projectService      project.Service
	repositoriesService repositories.Service
	managerService      v1ClaManager.IService
	easyCLAUserService  easyCLAUser.Service
	v2CompanyService    v2Company.Service
	eventService        events.Service
	projectCGRepo       projects_cla_groups.Repository
}

// Service interface
type Service interface {
	CreateCLAManager(claGroupID string, params cla_manager.CreateCLAManagerParams, authUsername string) (*models.CompanyClaManager, *models.ErrorResponse)
	DeleteCLAManager(claGroupID string, params cla_manager.DeleteCLAManagerParams) *models.ErrorResponse
	InviteCompanyAdmin(contactAdmin bool, companyID string, projectID string, userEmail string, name string, contributor *v1User.User, lFxPortalURL string) ([]*models.ClaManagerDesignee, error)
	CreateCLAManagerDesignee(companyID string, projectID string, userEmail string) (*models.ClaManagerDesignee, error)
	CreateCLAManagerRequest(contactAdmin bool, companyID string, projectID string, userEmail string, fullName string, authUser *auth.User, LfxPortalURL string) (*models.ClaManagerDesignee, error)
	NotifyCLAManagers(notifyCLAManagers *models.NotifyClaManagerList) error
	CreateCLAManagerDesigneeByGroup(params cla_manager.CreateCLAManagerDesigneeByGroupParams, projectCLAGroups []*projects_cla_groups.ProjectClaGroup, f logrus.Fields) ([]*models.ClaManagerDesignee, string, error)
}

// NewService returns instance of CLA Manager service
func NewService(compService company.IService, projService project.Service, mgrService v1ClaManager.IService, claUserService easyCLAUser.Service,
	repoService repositories.Service, v2CompService v2Company.Service,
	evService events.Service, projectCGroupRepo projects_cla_groups.Repository) Service {
	return &service{
		companyService:      compService,
		projectService:      projService,
		repositoriesService: repoService,
		managerService:      mgrService,
		easyCLAUserService:  claUserService,
		v2CompanyService:    v2CompService,
		eventService:        evService,
		projectCGRepo:       projectCGroupRepo,
	}
}

// CreateCLAManager creates Cla Manager
func (s *service) CreateCLAManager(claGroupID string, params cla_manager.CreateCLAManagerParams, authUsername string) (*models.CompanyClaManager, *models.ErrorResponse) {
	f := logrus.Fields{
		"functionName": "CreateCLAManager",
		"claGroupID":   claGroupID,
		"projectSFID":  params.ProjectSFID,
		"companySFID":  params.CompanySFID,
		"authUsername": authUsername,
		"xUserName":    params.XUSERNAME,
		"xEmail":       params.XEMAIL,
	}

	re := regexp.MustCompile(`^\w{1,30}$`)
	if !re.MatchString(*params.Body.FirstName) || !re.MatchString(*params.Body.LastName) {
		msg := "Firstname and last Name values should not exceed 30 characters in length"
		log.WithFields(f).Warn(msg)
		return nil, &models.ErrorResponse{
			Message: msg,
			Code:    "400",
		}
	}
	if *params.Body.UserEmail == "" {
		msg := "UserEmail cannot be empty"
		log.WithFields(f).Warn(msg)
		return nil, &models.ErrorResponse{
			Message: msg,
			Code:    "400",
		}
	}

	// Search for salesForce Company aka external Company
	log.WithFields(f).Debugf("Getting company by external ID : %s", params.CompanySFID)
	companyModel, companyErr := s.companyService.GetCompanyByExternalID(params.CompanySFID)
	if companyErr != nil || companyModel == nil {
		msg := buildErrorMessage("company lookup error", claGroupID, params, companyErr)
		log.WithFields(f).Warn(msg)
		return nil, &models.ErrorResponse{
			Message: msg,
			Code:    "400",
		}
	}

	claGroup, err := s.projectService.GetCLAGroupByID(claGroupID)
	if err != nil || claGroup == nil {
		msg := buildErrorMessage("cla group search by ID failure", claGroupID, params, err)
		log.WithFields(f).Warn(msg)
		return nil, &models.ErrorResponse{
			Message: msg,
			Code:    "400",
		}
	}
	// Get user by email
	userServiceClient := v2UserService.GetClient()
	// Get Manager lf account by username. Used for email content
	managerUser, mgrErr := userServiceClient.GetUserByUsername(authUsername)
	if mgrErr != nil || managerUser == nil {
		msg := fmt.Sprintf("Failed to get Lfx User with username : %s ", authUsername)
		log.WithFields(f).Warn(msg)
	}
	// GetSF Org
	orgClient := v2OrgService.GetClient()
	organizationSF, orgErr := orgClient.GetOrganization(params.CompanySFID)
	if orgErr != nil {
		msg := buildErrorMessage("organization service lookup error", claGroupID, params, orgErr)
		log.WithFields(f).Warn(msg)
		return nil, &models.ErrorResponse{
			Message: msg,
			Code:    "400",
		}
	}
	acsClient := v2AcsService.GetClient()
	user, userErr := userServiceClient.SearchUserByEmail(params.Body.UserEmail.String())

	if userErr != nil {
		designeeName := fmt.Sprintf("%s %s", *params.Body.FirstName, *params.Body.LastName)
		designeeEmail := params.Body.UserEmail.String()
		msg := fmt.Sprintf("User does not have an LF Login account and has been sent an email invite: %s.", *params.Body.UserEmail)
		log.WithFields(f).Warn(msg)
		sendEmailErr := sendEmailToUserWithNoLFID(claGroup.ProjectName, authUsername, *managerUser.Emails[0].EmailAddress, designeeName, designeeEmail, organizationSF.ID, nil, utils.CLAManagerRole)
		if sendEmailErr != nil {
			emailMessage := fmt.Sprintf("Failed to send email to user : %s ", designeeEmail)
			return nil, &models.ErrorResponse{
				Message: emailMessage,
				Code:    "400",
			}
		}
		return nil, &models.ErrorResponse{
			Message: msg,
			Code:    "400",
		}
	}

	// Check if user exists in easyCLA DB, if not add User
	log.WithFields(f).Debugf("Checking user: %+v in easyCLA records", user)
	claUser, claUserErr := s.easyCLAUserService.GetUserByLFUserName(user.Username)
	if claUserErr != nil {
		msg := fmt.Sprintf("Problem getting claUser by :%s, error: %+v ", user.Username, claUserErr)
		log.WithFields(f).Warn(msg)
		return nil, &models.ErrorResponse{
			Message: msg,
			Code:    "400",
		}
	}

	if claUser == nil {
		msg := fmt.Sprintf("User not found when searching by LF Login: %s and shall be created", user.Username)
		log.WithFields(f).Debug(msg)
		userName := fmt.Sprintf("%s %s", *params.Body.FirstName, *params.Body.LastName)
		_, currentTimeString := utils.CurrentTime()
		claUserModel := &v1Models.User{
			UserExternalID: params.CompanySFID,
			LfEmail:        *user.Emails[0].EmailAddress,
			Admin:          true,
			LfUsername:     user.Username,
			DateCreated:    currentTimeString,
			DateModified:   currentTimeString,
			Username:       userName,
			Version:        "v1",
		}
		newUserModel, userModelErr := s.easyCLAUserService.CreateUser(claUserModel, nil)
		if userModelErr != nil {
			msg := fmt.Sprintf("Failed to create user : %+v", claUserModel)
			log.WithFields(f).Warn(msg)
			return nil, &models.ErrorResponse{
				Message: msg,
				Code:    "400",
			}
		}
		log.WithFields(f).Debugf("Created easyCLAUser %+v ", newUserModel)
	}

	// GetSFProject
	ps := v2ProjectService.GetClient()
	projectSF, projectErr := ps.GetProject(params.ProjectSFID)
	if projectErr != nil {
		msg := buildErrorMessage("project service lookup error", claGroupID, params, projectErr)
		log.WithFields(f).Warn(msg)
		return nil, &models.ErrorResponse{
			Message: msg,
			Code:    "400",
		}
	}

	// Add CLA Manager to Database
	signature, addErr := s.managerService.AddClaManager(companyModel.CompanyID, claGroupID, user.Username)
	if addErr != nil {
		msg := buildErrorMessageCreate(params, addErr)
		log.WithFields(f).Warn(msg)
		return nil, &models.ErrorResponse{
			Message: msg,
			Code:    "400",
		}
	}
	if signature == nil {
		sigMsg := fmt.Sprintf("Signature not found for project: %s and company: %s ", claGroupID, companyModel.CompanyID)
		log.WithFields(f).Warn(sigMsg)
		return nil, &models.ErrorResponse{
			Message: sigMsg,
			Code:    "400",
		}
	}

	log.WithFields(f).Debug("Getting role")
	// Get RoleID for cla-manager

	roleID, roleErr := acsClient.GetRoleID(utils.CLAManagerRole)
	if roleErr != nil {
		msg := buildErrorMessageCreate(params, roleErr)
		log.WithFields(f).Warn(msg)
		return nil, &models.ErrorResponse{
			Message: msg,
			Code:    "400",
		}
	}
	log.WithFields(f).Debugf("Role ID for %s: %s", utils.CLAManagerRole, roleID)
	log.WithFields(f).Debugf("Creating user role Scope for user: %s ", *params.Body.UserEmail)

	hasScope, err := orgClient.IsUserHaveRoleScope(utils.CLAManagerRole, user.ID, params.CompanySFID, params.ProjectSFID)
	if err != nil {
		msg := buildErrorMessageCreate(params, err)
		log.WithFields(f).Warn(msg)
		return nil, &models.ErrorResponse{
			Message: msg,
			Code:    "400",
		}
	}
	if hasScope {
		msg := fmt.Sprintf("User %s is already %s for Company: %s and Project: %s",
			user.Username, utils.CLAManagerRole, params.CompanySFID, params.ProjectSFID)
		log.WithFields(f).Warn(msg)
		return nil, &models.ErrorResponse{
			Message: msg,
			Code:    "409",
		}
	}

	projectCLAGroups, getErr := s.projectCGRepo.GetProjectsIdsForClaGroup(claGroupID)
	log.WithFields(f).Debugf("Getting associated SF projects for claGroup: %s ", claGroupID)

	if getErr != nil {
		msg := buildErrorMessageCreate(params, getErr)
		log.WithFields(f).Warn(msg)
		return nil, &models.ErrorResponse{
			Message: msg,
			Code:    "400",
		}
	}

	// Check if Project is signed at Foundation or Project Level
	signedAtFoundation, signedErr := s.projectService.SignedAtFoundationLevel(claGroupID)

	if signedErr != nil {
		msg := buildErrorMessageCreate(params, signedErr)
		log.WithFields(f).Warn(msg)
		return nil, &models.ErrorResponse{
			Message: msg,
			Code:    "400",
		}
	}

	if signedAtFoundation {
		foundationSFID := projectCLAGroups[0].FoundationSFID
		scopeErr := orgClient.CreateOrgUserRoleOrgScopeProjectOrg(params.Body.UserEmail.String(), foundationSFID, params.CompanySFID, roleID)
		if scopeErr != nil {
			msg := buildErrorMessageCreate(params, scopeErr)
			log.WithFields(f).Warn(msg)
			return nil, &models.ErrorResponse{
				Message: msg,
				Code:    "400",
			}
		}

	} else {
		for _, projectCG := range projectCLAGroups {
			scopeErr := orgClient.CreateOrgUserRoleOrgScopeProjectOrg(params.Body.UserEmail.String(), projectCG.ProjectSFID, params.CompanySFID, roleID)
			if scopeErr != nil {
				msg := buildErrorMessageCreate(params, scopeErr)
				log.WithFields(f).Warn(msg)
				return nil, &models.ErrorResponse{
					Message: msg,
					Code:    "400",
				}
			}
		}
	}

	if user.Type == utils.Lead {
		// convert user to contact
		log.WithFields(f).Debug("converting lead to contact")
		err := userServiceClient.ConvertToContact(user.ID)
		if err != nil {
			msg := fmt.Sprintf("converting lead to contact failed: %v", err)
			log.WithFields(f).Warn(msg)
			return nil, &models.ErrorResponse{
				Message: msg,
				Code:    "400",
			}
		}
	}

	claCompanyManager := &models.CompanyClaManager{
		LfUsername:       user.Username,
		Email:            *params.Body.UserEmail,
		UserSfid:         user.ID,
		ApprovedOn:       time.Now().String(),
		ProjectSfid:      params.ProjectSFID,
		ClaGroupName:     claGroup.ProjectName,
		ProjectID:        claGroupID,
		ProjectName:      projectSF.Name,
		OrganizationName: companyModel.CompanyName,
		OrganizationSfid: params.CompanySFID,
		Name:             fmt.Sprintf("%s %s", user.FirstName, user.LastName),
	}
	return claCompanyManager, nil
}

func (s *service) DeleteCLAManager(claGroupID string, params cla_manager.DeleteCLAManagerParams) *models.ErrorResponse {
	f := logrus.Fields{
		"functionName": "DeleteCLAManager",
		"projectSFID":  params.ProjectSFID,
		"companySFID":  params.CompanySFID,
		"xUserName":    params.XUSERNAME,
		"xEmail":       params.XEMAIL,
	}
	// Get user by firstname,lastname and email parameters
	userServiceClient := v2UserService.GetClient()
	user, userErr := userServiceClient.GetUserByUsername(params.UserLFID)

	if userErr != nil {
		msg := fmt.Sprintf("Failed to get user when searching by username: %s , error: %v ", params.UserLFID, userErr)
		log.WithFields(f).Warn(msg)
		return &models.ErrorResponse{
			Message: msg,
			Code:    "400",
		}
	}

	// Search for salesForce Company aka external Company
	companyModel, companyErr := s.companyService.GetCompanyByExternalID(params.CompanySFID)
	if companyErr != nil || companyModel == nil {
		msg := buildErrorMessageDelete(params, companyErr)
		log.WithFields(f).Warn(msg)
		return &models.ErrorResponse{
			Message: msg,
			Code:    "400",
		}
	}

	acsClient := v2AcsService.GetClient()

	roleID, roleErr := acsClient.GetRoleID("cla-manager")
	if roleErr != nil {
		msg := buildErrorMessageDelete(params, roleErr)
		log.WithFields(f).Warn(msg)
		return &models.ErrorResponse{
			Message: msg,
			Code:    "400",
		}
	}
	log.WithFields(f).Debugf("Role ID for cla-manager-role : %s", roleID)

	projectCLAGroups, getErr := s.projectCGRepo.GetProjectsIdsForClaGroup(claGroupID)

	if getErr != nil {
		msg := buildErrorMessageDelete(params, getErr)
		log.WithFields(f).Warn(msg)
		return &models.ErrorResponse{
			Message: msg,
			Code:    "400",
		}
	}

	orgClient := v2OrgService.GetClient()

	for _, projectCG := range projectCLAGroups {
		scopeID, scopeErr := orgClient.GetScopeID(params.CompanySFID, projectCG.ProjectSFID, "cla-manager", "project|organization", params.UserLFID)
		if scopeErr != nil {
			msg := buildErrorMessageDelete(params, scopeErr)
			log.WithFields(f).Warn(msg)
			return &models.ErrorResponse{
				Message: msg,
				Code:    "400",
			}
		}
		if scopeID == "" {
			msg := buildErrorMessageDelete(params, ErrScopeNotFound)
			log.WithFields(f).Warn(msg)
			return &models.ErrorResponse{
				Message: msg,
				Code:    "400",
			}
		}
		email := *user.Emails[0].EmailAddress
		deleteErr := orgClient.DeleteOrgUserRoleOrgScopeProjectOrg(params.CompanySFID, roleID, scopeID, &user.Username, &email)
		if deleteErr != nil {
			msg := buildErrorMessageDelete(params, deleteErr)
			log.WithFields(f).Warn(msg)
			return &models.ErrorResponse{
				Message: msg,
				Code:    "400",
			}
		}
	}

	signature, deleteErr := s.managerService.RemoveClaManager(companyModel.CompanyID, claGroupID, params.UserLFID)

	if deleteErr != nil {
		msg := buildErrorMessageDelete(params, deleteErr)
		log.WithFields(f).Warn(msg)
		return &models.ErrorResponse{
			Message: msg,
			Code:    "400",
		}
	}
	if signature == nil {
		msg := fmt.Sprintf("Not found signature for project: %s and company: %s ", claGroupID, companyModel.CompanyID)
		log.WithFields(f).Warn(msg)
		return &models.ErrorResponse{
			Message: msg,
			Code:    "400",
		}
	}

	return nil
}

//CreateCLAManagerDesignee creates designee for cla manager prospect
func (s *service) CreateCLAManagerDesignee(companySFID string, projectSFID string, userEmail string) (*models.ClaManagerDesignee, error) {
	f := logrus.Fields{
		"functionName": "CreateCLAManagerDesignee",
		"companySFID":  companySFID,
		"projectSFID":  projectSFID,
		"userEmail":    userEmail,
	}
	// integrate user,acs,org and project services
	userClient := v2UserService.GetClient()
	acServiceClient := v2AcsService.GetClient()
	orgClient := v2OrgService.GetClient()
	projectClient := v2ProjectService.GetClient()

	log.WithFields(f).Debugf("loading company by external ID...")
	v1CompanyModel, companyErr := s.companyService.GetCompanyByExternalID(companySFID)
	if companyErr != nil {
		log.WithFields(f).Warnf("company not found, error: %+v", companyErr)
		return nil, companyErr
	}

	log.WithFields(f).Debugf("checking if company/project is signed with CLA managers...")
	isSigned, signedErr := s.isSigned(v1CompanyModel, projectSFID)
	if signedErr != nil {
		msg := fmt.Sprintf("EasyCLA - 400 Bad Request - %s", signedErr)
		log.WithFields(f).Warn(msg)
		return nil, signedErr
	}

	if isSigned {
		msg := fmt.Sprintf("EasyCLA - 400 Bad Request - Project: %s is already signed", projectSFID)
		log.WithFields(f).Warn(msg)
		return nil, ErrProjectSigned
	}

	userService := v2UserService.GetClient()
	log.WithFields(f).Debug("searching user in user service...")
	// This routine is taking 24-29 seconds when running locally -> User service in DEV
	//lfxUser, userErr := userService.SearchUserByEmail(userEmail)
	// This routine is taking 4 seconds when running locally -> User service in DEV
	lfxUser, userErr := userService.SearchUsersByEmail(userEmail)
	if userErr != nil {
		log.WithFields(f).Debugf("Failed to get user by email: %s, error: %+v", userEmail, userErr)
		return nil, ErrLFXUserNotFound
	}

	log.WithFields(f).Debugf("checking if user has %s role scope...", utils.CLADesigneeRole)
	// Check if user is already CLA Manager designee of project|organization scope
	hasRoleScope, hasRoleScopeErr := orgClient.IsUserHaveRoleScope(utils.CLADesigneeRole, lfxUser.ID, companySFID, projectSFID)
	if hasRoleScopeErr != nil {
		// Skip 404 for ListOrgUsrServiceScopes endpoint
		if _, ok := hasRoleScopeErr.(*organizations.ListOrgUsrServiceScopesNotFound); !ok {
			log.WithFields(f).Debugf("Failed to check roleScope: %s for user: %s", utils.CLADesigneeRole, lfxUser.Username)
			return nil, hasRoleScopeErr
		}
	}
	if hasRoleScope {
		log.WithFields(f).Warnf("Conflict - user has role scope: %s", utils.CLADesigneeRole)
		return nil, ErrCLAManagerDesigneeConflict
	}

	log.WithFields(f).Debug("loading project by SFID...")
	projectSF, projectErr := projectClient.GetProject(projectSFID)
	if projectErr != nil {
		log.WithFields(f).Debugf("problem getting project: %s from the project service, error: %+v", projectSFID, projectErr)
		return nil, projectErr
	}

	log.WithFields(f).Debugf("loading role ID for %s...", utils.CLADesigneeRole)
	roleID, designeeErr := acServiceClient.GetRoleID(utils.CLADesigneeRole)
	if designeeErr != nil {
		log.WithFields(f).Warnf("Problem getting role ID for cla-manager-designee, error: %+v", designeeErr)
		return nil, designeeErr
	}

	log.WithFields(f).Debugf("creating user role organization scope for user: %s, with role: %s with role ID: %s using project|org: %s|%s...",
		userEmail, utils.CLADesigneeRole, roleID, projectSFID, companySFID)
	scopeErr := orgClient.CreateOrgUserRoleOrgScopeProjectOrg(userEmail, projectSFID, companySFID, roleID)
	if scopeErr != nil {
		msg := fmt.Sprintf("Problem creating projectOrg scope for email: %s , projectSFID: %s, companyID: %s", userEmail, projectSFID, companySFID)
		log.Warn(msg)
		if _, ok := scopeErr.(*organizations.CreateOrgUsrRoleScopesConflict); ok {
			return nil, ErrRoleScopeConflict
		}
		return nil, scopeErr
	}
	log.WithFields(f).Debugf("created user role organization scope for user: %s, with role: %s with role ID: %s using project|org: %s|%s...",
		userEmail, utils.CLADesigneeRole, roleID, projectSFID, companySFID)

	// Log Event
	s.eventService.LogEvent(
		&events.LogEventArgs{
			EventType:         events.AssignUserRoleScopeType,
			LfUsername:        lfxUser.Username,
			ExternalProjectID: projectSFID,
			CompanyModel:      v1CompanyModel,
			CompanyID:         v1CompanyModel.CompanyID,
			UserModel:         &v1Models.User{LfUsername: lfxUser.Username, UserID: lfxUser.ID},
			EventData: &events.AssignRoleScopeData{
				Role:  "cla-manager-designee",
				Scope: fmt.Sprintf("%s|%s", projectSFID, companySFID),
			},
		})

	if lfxUser.Type == utils.Lead {
		log.Debugf("Converting user: %s from lead to contact ", userEmail)
		contactErr := userClient.ConvertToContact(lfxUser.ID)
		if contactErr != nil {
			log.Debugf("failed to convert user: %s to contact ", userEmail)
			return nil, contactErr
		}
		// Log user conversion event
		s.eventService.LogEvent(&events.LogEventArgs{
			EventType:         events.ConvertUserToContactType,
			LfUsername:        lfxUser.Username,
			ExternalProjectID: projectSFID,
			EventData:         &events.UserConvertToContactData{},
		})
	}

	claManagerDesignee := &models.ClaManagerDesignee{
		LfUsername:  lfxUser.Username,
		UserSfid:    lfxUser.ID,
		Type:        lfxUser.Type,
		AssignedOn:  time.Now().String(),
		Email:       strfmt.Email(userEmail),
		ProjectSfid: projectSFID,
		CompanySfid: companySFID,
		ProjectName: projectSF.Name,
	}

	return claManagerDesignee, nil
}

//CreateCLAManagerDesigneeByGroup creates designee by group for cla manager prospect
func (s *service) CreateCLAManagerDesigneeByGroup(params cla_manager.CreateCLAManagerDesigneeByGroupParams, projectCLAGroups []*projects_cla_groups.ProjectClaGroup, f logrus.Fields) ([]*models.ClaManagerDesignee, string, error) {
	orgClient := v2OrgService.GetClient()
	userEmail := params.Body.UserEmail.String()

	var designeeScopes []*models.ClaManagerDesignee

	org, orgErr := orgClient.GetOrganization(params.CompanySFID)
	if orgErr != nil {
		msg := fmt.Sprintf("Getting organization by ID: %s failed", params.CompanySFID)
		return nil, msg, orgErr
	}

	// Assign company owner role
	// Set Company owner role for cla manager designee

	err := s.setOwnerRole(userEmail, org.ID)
	if err != nil {
		msg := fmt.Sprintf("Problem assigning company owner role for user: %s and organization: %s , error: %+v", userEmail, org.ID, err)
		log.WithFields(f).Warn(msg)
		return nil, msg, err
	}

	claGroupID := projectCLAGroups[0].ClaGroupID
	signedAtFoundationLevel, signedErr := s.projectService.SignedAtFoundationLevel(claGroupID)
	if signedErr != nil {
		msg := fmt.Sprintf("Problem getting level of CLA Group Signature for claGroup: %s ", claGroupID)
		return nil, msg, signedErr
	}

	if signedAtFoundationLevel {
		foundationSFID := projectCLAGroups[0].FoundationSFID
		if foundationSFID != "" {
			claManagerDesignee, err := s.CreateCLAManagerDesignee(params.CompanySFID, foundationSFID, params.Body.UserEmail.String())
			if err != nil {
				if err == ErrCLAManagerDesigneeConflict {
					msg := fmt.Sprintf("Conflict assigning cla manager role for Foundation SFID: %s ", foundationSFID)
					return nil, msg, err
				}
				msg := fmt.Sprintf("Creating cla manager failed for Foundation SFID: %s ", foundationSFID)
				return nil, msg, err
			}
			designeeScopes = append(designeeScopes, claManagerDesignee)
		}
	} else {
		for _, pcg := range projectCLAGroups {
			log.WithFields(f).Debugf("creating CLA Manager Designee for Project SFID: %s", pcg.ProjectSFID)
			claManagerDesignee, err := s.CreateCLAManagerDesignee(params.CompanySFID, pcg.ProjectSFID, params.Body.UserEmail.String())
			if err != nil {
				if err == ErrCLAManagerDesigneeConflict {
					msg := fmt.Sprintf("Conflict assigning cla manager role for Project SFID: %s, error: %s ", pcg.ProjectSFID, err)
					return nil, msg, err
				}
				msg := fmt.Sprintf("Creating cla manager failed for Project SFID: %s, error: %s ", pcg.ProjectSFID, err)
				return nil, msg, err
			}
			designeeScopes = append(designeeScopes, claManagerDesignee)
		}
	}

	return designeeScopes, "", nil
}

// CreateCLAManagerRequest service method
func (s *service) CreateCLAManagerRequest(contactAdmin bool, companySFID string, projectID string, userEmail string, fullName string, authUser *auth.User, LfxPortalURL string) (*models.ClaManagerDesignee, error) {
	f := logrus.Fields{
		"functionName":  "CreateCLAManagerRequest",
		"contactAdmin":  contactAdmin,
		"companySFID":   companySFID,
		"projectID":     projectID,
		"userEmail":     userEmail,
		"fullName":      fullName,
		"authUserName":  authUser.UserName,
		"authUserEmail": authUser.Email,
	}

	orgService := v2OrgService.GetClient()

	log.WithFields(f).Debugf("loading company by external ID...")
	// Search for salesForce Company aka external Company
	v1CompanyModel, companyErr := s.companyService.GetCompanyByExternalID(companySFID)
	if companyErr != nil {
		msg := fmt.Sprintf("EasyCLA - 400 Bad Request - %s", companyErr)
		log.Warn(msg)
		return nil, companyErr
	}

	// Determine if the CCLA is already signed or not
	log.WithFields(f).Debugf("checking if company/project is signed with CLA managers...")
	isSigned, signedErr := s.isSigned(v1CompanyModel, projectID)
	if signedErr != nil {
		msg := fmt.Sprintf("EasyCLA - 400 Bad Request - %s", signedErr)
		log.WithFields(f).Warn(msg)
		return nil, signedErr
	}

	if isSigned {
		msg := fmt.Sprintf("EasyCLA - 400 Bad Request - Project: %s is already signed ", projectID)
		log.WithFields(f).Warn(msg)
		return nil, ErrProjectSigned
	}

	log.WithFields(f).Debugf("querying project service for project details...")
	// GetSFProject
	ps := v2ProjectService.GetClient()
	projectSF, projectErr := ps.GetProject(projectID)
	if projectErr != nil {
		msg := fmt.Sprintf("EasyCLA - 400 Bad Request - Project service lookup error for SFID: %s, error : %+v",
			projectID, projectErr)
		log.WithFields(f).Warn(msg)
		return nil, projectErr
	}

	// Check if sending cla manager request to company admin
	if contactAdmin {
		log.WithFields(f).Debug("sending email to company Admin")
		log.WithFields(f).Debug("querying user admin scopes...")
		scopes, listScopeErr := orgService.ListOrgUserAdminScopes(companySFID, nil)
		if listScopeErr != nil {
			msg := fmt.Sprintf("EasyCLA - 400 Bad Request - Admin lookup error for organisation SFID: %s, error: %+v ",
				companySFID, listScopeErr)
			log.WithFields(f).Warn(msg)
			return nil, listScopeErr
		}

		if len(scopes.Userroles) == 0 {
			msg := fmt.Sprintf("EasyCLA - 404 NotFound - No admins for organization SFID: %s",
				companySFID)
			log.WithFields(f).Warn(msg)
			return nil, ErrNoOrgAdmins
		}

		for _, admin := range scopes.Userroles {
			log.WithFields(f).Debugf("sending email to organization admin: %+v", admin)
			sendEmailToOrgAdmin(admin.Contact.EmailAddress, admin.Contact.Name, v1CompanyModel.CompanyName, []string{projectSF.Name}, authUser.Email, authUser.UserName, LfxPortalURL)
			// Make a note in the event log
			s.eventService.LogEvent(&events.LogEventArgs{
				EventType:         events.ContributorNotifyCompanyAdminType,
				LfUsername:        authUser.UserName,
				ExternalProjectID: projectID,
				CompanyID:         v1CompanyModel.CompanyID,
				EventData: &events.ContributorNotifyCompanyAdminData{
					AdminName:  admin.Contact.Name,
					AdminEmail: admin.Contact.EmailAddress,
				},
			})
		}

		return nil, nil
	}
	log.WithFields(f).Debug("not sending admin email...")

	userService := v2UserService.GetClient()
	log.WithFields(f).Debug("searching user in user service...")
	// This routine is taking 24-29 seconds when running locally -> User service in DEV
	//lfxUser, userErr := userService.SearchUserByEmail(userEmail)
	// This routine is taking 4 seconds when running locally -> User service in DEV
	lfxUser, userErr := userService.SearchUsersByEmail(userEmail)
	if userErr != nil {
		msg := fmt.Sprintf("User: %s does not have an LF Login", userEmail)
		log.WithFields(f).Warn(msg)
		// Send email
		sendEmailErr := sendEmailToUserWithNoLFID(projectSF.Name, authUser.UserName, authUser.Email, fullName, userEmail, v1CompanyModel.CompanyID, &projectSF.ID, utils.CLADesigneeRole)
		if sendEmailErr != nil {
			log.WithFields(f).Warnf("Error sending email: %+v", sendEmailErr)
			return nil, sendEmailErr
		}
		return nil, ErrNoLFID
	}

	log.WithFields(f).Debug("sending CLA manager designee request...")
	claManagerDesignee, err := s.CreateCLAManagerDesignee(companySFID, projectID, userEmail)
	if err != nil {
		// Check conflict for role scope
		if _, ok := err.(*organizations.CreateOrgUsrRoleScopesConflict); ok {
			log.WithFields(f).Warn("problem creating organization role scope for designee - role exists")
			return nil, ErrRoleScopeConflict
		}
		log.WithFields(f).Warnf("problem creating organization role scope for designee, error: %+v", err)
		return nil, err
	}

	log.WithFields(f).Debug("creating a contributor assigned CLA designee log event...")
	// Make a note in the event log
	s.eventService.LogEvent(&events.LogEventArgs{
		EventType:         events.ContributorAssignCLADesigneeType,
		LfUsername:        authUser.UserName,
		ExternalProjectID: projectID,
		CompanyID:         v1CompanyModel.CompanyID,
		EventData: &events.ContributorAssignCLADesignee{
			DesigneeName:  claManagerDesignee.LfUsername,
			DesigneeEmail: claManagerDesignee.Email.String(),
		},
	})

	log.WithFields(f).Debugf("sending Email to CLA Manager Designee email: %s ", userEmail)
	designeeName := fmt.Sprintf("%s %s", lfxUser.FirstName, lfxUser.LastName)
	sendEmailToCLAManagerDesignee(LfxPortalURL, v1CompanyModel.CompanyName, []string{projectSF.Name}, userEmail, designeeName, authUser.Email, authUser.UserName)

	log.WithFields(f).Debug("creating a contributor notify CLA designee log event...")
	// Make a note in the event log
	s.eventService.LogEvent(&events.LogEventArgs{
		EventType:         events.ContributorNotifyCLADesigneeType,
		LfUsername:        authUser.UserName,
		ExternalProjectID: projectID,
		CompanyID:         v1CompanyModel.CompanyID,
		EventData: &events.ContributorNotifyCLADesignee{
			DesigneeName:  claManagerDesignee.LfUsername,
			DesigneeEmail: claManagerDesignee.Email.String(),
		},
	})

	log.WithFields(f).Debugf("CLA Manager designee created: %+v", claManagerDesignee)
	return claManagerDesignee, nil
}

func (s *service) InviteCompanyAdmin(contactAdmin bool, companyID string, projectID string, userEmail string, name string, contributor *v1User.User, LfxPortalURL string) ([]*models.ClaManagerDesignee, error) {
	orgService := v2OrgService.GetClient()
	projectService := v2ProjectService.GetClient()
	userService := v2UserService.GetClient()
	f := logrus.Fields{"companyID": companyID,
		"claGroupID": projectID,
		"userEmail":  userEmail,
		"name":       name}

	// Get project cla Group records
	log.WithFields(f).Debugf("Getting SalesForce Projects for claGroup: %s ", projectID)
	projectCLAGroups, getErr := s.projectCGRepo.GetProjectsIdsForClaGroup(projectID)
	if getErr != nil {
		msg := fmt.Sprintf("Error getting SF projects for claGroup: %s ", projectID)
		log.Debug(msg)
	}

	// Get company
	log.WithFields(f).Debugf("Get company for companyID: %s ", companyID)
	companyModel, companyErr := s.companyService.GetCompany(companyID)
	if companyErr != nil || companyModel.CompanyExternalID == "" {
		msg := fmt.Sprintf("Problem getting company for companyID: %s ", companyID)
		log.Warn(msg)
		if companyErr.Error() == "company does not exist" {
			return nil, ErrCLACompanyNotFound
		}
		return nil, companyErr
	}

	log.WithFields(f).Debugf("Getting CLA Project")
	project, projErr := s.projectService.GetCLAGroupByID(projectID)
	if projErr != nil {
		msg := fmt.Sprintf("Unable to get CLA Project: %s, error: %+v ", projectID, projErr)
		log.WithFields(f).Warnf("unable to get claGroup")
		log.Warn(msg)
		return nil, projErr
	}

	organization, orgErr := orgService.GetOrganization(companyModel.CompanyExternalID)
	if orgErr != nil {
		msg := fmt.Sprintf("Problem getting company by ID: %s ", companyID)
		log.Warn(msg)
		return nil, orgErr
	}

	// Get suggested CLA Manager user details
	user, userErr := userService.SearchUserByEmail(userEmail)
	if userErr != nil {
		msg := fmt.Sprintf("UserEmail: %s has no LF Login and has been sent an invite email to create an account , error: %+v", userEmail, userErr)
		log.Warn(msg)
		// Send Email
		var contributorEmail *string
		if len(contributor.UserEmails) > 0 {
			contributorEmail = &contributor.UserEmails[0]
		} else {
			contributorEmail = &contributor.LFEmail
		}

		// Use FoundationSFID
		foundationSFID := projectCLAGroups[0].FoundationSFID
		sendErr := sendEmailToUserWithNoLFID(project.ProjectName, contributor.UserName, *contributorEmail, name, userEmail, organization.ID, &foundationSFID, "cla-manager-designee")
		if sendErr != nil {
			return nil, sendErr
		}
		return nil, ErrNoLFID
	}
	var projectSFs []string
	for _, pcg := range projectCLAGroups {
		log.WithFields(f).Debugf("Getting salesforce project by SFID: %s ", pcg.ProjectSFID)
		projectSF, projectErr := projectService.GetProject(pcg.ProjectSFID)
		if projectErr != nil {
			msg := fmt.Sprintf("Problem getting salesforce Project ID: %s", pcg.ProjectSFID)
			log.WithFields(f).Warn(msg)
			return nil, projectErr
		}
		projectSFs = append(projectSFs, projectSF.Name)
	}

	// Set Company owner role for cla manager designee
	err := s.setOwnerRole(userEmail, organization.ID)
	if err != nil {
		msg := fmt.Sprintf("Problem assigning company owner role for user: %s and organization: %s , error: %+v", userEmail, organization.ID, err)
		log.WithFields(f).Warn(msg)
		return nil, err
	}

	var designeeScopes []*models.ClaManagerDesignee

	// Check if sending cla manager request to company admin
	if contactAdmin {
		log.Debugf("Sending email to company Admin")
		scopes, listScopeErr := orgService.ListOrgUserAdminScopes(companyModel.CompanyExternalID, nil)
		if listScopeErr != nil {
			msg := fmt.Sprintf("Admin lookup error for organisation SFID: %s ", companyModel.CompanyExternalID)
			log.WithFields(f).Warn(msg)
			return nil, listScopeErr
		}
		// Search for Easy CLA User
		log.Debugf("Getting user by ID: %s", contributor.UserID)
		userModel, userErr := s.easyCLAUserService.GetUser(contributor.UserID)
		if userErr != nil {
			msg := fmt.Sprintf("Problem getting user by ID: %s ", contributor.UserID)
			log.Warn(msg)
			return nil, userErr
		}

		for _, admin := range scopes.Userroles {
			// Check if is Gerrit User or GH User
			contributorEmailToOrgAdmin(admin.Contact.EmailAddress, admin.Contact.Name, organization.Name, projectSFs, userModel, LfxPortalURL)
			designeeScope := models.ClaManagerDesignee{
				Email: strfmt.Email(admin.Contact.EmailAddress),
				Name:  admin.Contact.Name,
			}
			designeeScopes = append(designeeScopes, &designeeScope)
		}
		return designeeScopes, nil
	}

	for _, pcg := range projectCLAGroups {
		log.WithFields(f).Debugf("Create cla manager designee for Project SFID: %s", pcg.ProjectSFID)
		claManagerDesignee, err := s.CreateCLAManagerDesignee(organization.ID, pcg.ProjectSFID, userEmail)
		if err != nil {
			msg := fmt.Sprintf("Problem creating cla Manager Designee for user : %s, error: %+v ", userEmail, err)
			log.WithFields(f).Warn(msg)
			return nil, err
		}
		designeeScopes = append(designeeScopes, claManagerDesignee)
	}

	log.Debugf("Sending Email to CLA Manager Designee email: %s ", userEmail)

	if contributor.LFUsername != "" && contributor.LFEmail != "" && len(projectSFs) > 0 {
		sendEmailToCLAManagerDesignee(LfxPortalURL, organization.Name, projectSFs, userEmail, user.Name, contributor.LFEmail, contributor.LFUsername)
	} else {
		contributorUserName, contributorEmail := getContributorPublicEmail(contributor)
		sendEmailToCLAManagerDesignee(LfxPortalURL, organization.Name, projectSFs, userEmail, user.Name, contributorUserName, contributorEmail)
	}

	log.Debugf("CLA Manager designee created : %+v", designeeScopes)
	return designeeScopes, nil

}

func (s *service) NotifyCLAManagers(notifyCLAManagers *models.NotifyClaManagerList) error {
	// Search for Easy CLA User
	log.Debugf("Getting user by ID: %s", notifyCLAManagers.UserID)
	userModel, userErr := s.easyCLAUserService.GetUser(notifyCLAManagers.UserID)
	if userErr != nil {
		msg := fmt.Sprintf("Problem getting user by ID: %s ", notifyCLAManagers.UserID)
		log.Warn(msg)
		return ErrCLAUserNotFound
	}

	log.Debugf("Sending notification emails to claManagers: %+v", notifyCLAManagers.List)
	for _, claManager := range notifyCLAManagers.List {
		sendEmailToCLAManager(claManager.Name, claManager.Email.String(), userModel, notifyCLAManagers.CompanyName, notifyCLAManagers.ClaGroupName)
	}

	return nil
}

// Utility function that sets company owner role
func (s *service) setOwnerRole(userEmail string, organizationID string) error {
	orgClient := v2OrgService.GetClient()
	acsClient := v2AcsService.GetClient()
	userClient := v2UserService.GetClient()
	user, err := userClient.SearchUserByEmail(userEmail)
	if err != nil {
		msg := fmt.Sprintf("Failed searching user by email :%s ", userEmail)
		log.Warn(msg)
		return err
	}

	log.Info(fmt.Sprintf("Check if user : %s is a company owner ", userEmail))
	var hasOwnerScope bool
	if user.Account.Name == NoAccount {
		// flag company owner scope if user is not associated with an org
		hasOwnerScope = false
	} else {
		// Check if user is in organization
		var userOrg string
		if user.Account.ID != organizationID {
			userOrg = user.Account.ID
		} else {
			userOrg = organizationID
		}
		log.Info(fmt.Sprintf("Checking company-owner against company: %s ", userOrg))
		hasOwnerScope, err = orgClient.IsCompanyOwner(user.ID, userOrg)
		if err != nil {
			return err
		}
	}

	log.Info(fmt.Sprintf("User :%s isCompanyOwner: %t", userEmail, hasOwnerScope))

	if !hasOwnerScope {
		companyOwner := "company-owner"
		// Check if company has company owner
		_, scopeErr := orgClient.ListOrgUserAdminScopes(organizationID, &companyOwner)
		if scopeErr != nil {
			// Only assign if company owner doesnt exist
			if _, ok := scopeErr.(*organizations.ListOrgUsrAdminScopesNotFound); ok {
				//Get Role ID
				roleID, designeeErr := acsClient.GetRoleID("company-owner")
				if designeeErr != nil {
					msg := "Problem getting role ID for company-owner"
					log.Warn(msg)
					return designeeErr
				}

				err := orgClient.CreateOrgUserRoleOrgScope(userEmail, organizationID, roleID)
				if err != nil {
					log.Warnf("Organization Service - Failed to assign company-owner role to user: %s, error: %+v ", userEmail, err)
					return err
				}
				// When role is assigned successfully skip 404 issue
				return nil
			}
			return scopeErr
		}
	}

	return nil
}

func sendEmailToCLAManager(manager string, managerEmail string, userModel *v1Models.User, company string, claGroupName string) {
	subject := fmt.Sprintf("EasyCLA: Approval Request for contributor: %s", getBestUserName(userModel))
	recipients := []string{managerEmail}
	body := fmt.Sprintf(`
	<p>Hello %s,</p>
	<p>This is a notification email from EasyCLA regarding the organization %s.</p>
	<p>The following contributor would like to submit a contribution to the %s CLA Group
	   and is requesting to be approved as a contributor for your organization: </p>
	<p>%s</p>
	<p>Please notify the contributor once they are added so that they may complete the contribution process.</p>
	%s
    %s`,
		manager, company, claGroupName, getFormattedUserDetails(userModel),
		utils.GetEmailHelpContent(true), utils.GetEmailSignOffContent())
	err := utils.SendEmail(subject, body, recipients)
	if err != nil {
		log.Warnf("problem sending email with subject: %s to recipients: %+v, error: %+v", subject, recipients, err)
	} else {
		log.Debugf("sent email with subject: %s to recipients: %+v", subject, recipients)
	}
}

// getBestUserName is a helper function to extract what information we can from the user record for purposes of displaying the user's name
func getBestUserName(model *v1Models.User) string {
	if model.Username != "" {
		return model.Username
	}

	if model.GithubUsername != "" {
		return model.GithubUsername
	}

	if model.LfUsername != "" {
		return model.LfUsername
	}

	return "User Name Unknown"
}

func getContributorPublicEmail(model *v1User.User) (string, string) {
	var contributorUserName, contributorEmail string
	if model.LFUsername != "" {
		contributorUserName = model.LFUsername
	}

	if model.LFEmail != "" {
		contributorEmail = model.LFEmail
	}

	if contributorUserName == "" {
		contributorUserName = model.UserGithubUsername
	}

	if contributorEmail == "" && len(model.UserEmails) > 0 {
		for _, email := range model.UserEmails {
			if strings.Contains(email, "users.noreply.github.com") {
				continue
			}
			contributorEmail = email
		}
	}
	return contributorUserName, contributorEmail
}

// getFormattedUserDetails is a helper function to extract what information we can from the user record for purposes of displaying the user's information
func getFormattedUserDetails(model *v1Models.User) string {
	var details []string
	if model.Username != "" {
		details = append(details, fmt.Sprintf("User Name: %s", model.Username))
	}

	if model.GithubUsername != "" {
		details = append(details, fmt.Sprintf("GitHub User Name: %s", model.GithubUsername))
	}

	if model.GithubID != "" {
		details = append(details, fmt.Sprintf("GitHub ID: %s", model.GithubID))
	}

	if model.LfUsername != "" {
		details = append(details, fmt.Sprintf("LF Login: %s", model.LfUsername))
	}

	if model.LfEmail != "" {
		details = append(details, fmt.Sprintf("LF Email: %s", model.LfEmail))
	}

	if model.Emails != nil {
		details = append(details, fmt.Sprintf("Emails: %s", strings.Join(model.Emails, ", ")))
	}

	return strings.Join(details, ",")
}

// isSigned is a helper function to check if project/claGroup is signed
func (s *service) isSigned(companyModel *v1Models.Company, projectID string) (bool, error) {
	f := logrus.Fields{
		"functionName": "isSigned",
		"companyID":    companyModel.CompanyID,
		"companyName":  companyModel.CompanyName,
		"companySFID":  companyModel.CompanyExternalID,
		"projectID":    projectID,
	}

	f["companyID"] = companyModel.CompanyID
	f["companyName"] = companyModel.CompanyName
	log.WithFields(f).Debug("loading CLA Managers for company/project")
	claManagers, err := s.v2CompanyService.GetCompanyProjectCLAManagers(companyModel.CompanyID, projectID)
	if err != nil {
		msg := fmt.Sprintf("EasyCLA - 400 Bad Request : %v", err)
		log.WithFields(f).Warn(msg)
		return false, err
	}

	if len(claManagers.List) > 0 {
		log.WithFields(f).Warnf("CLA Group signed for company/project - %d CLA Managers", len(claManagers.List))
		return true, nil
	}

	return false, nil
}

func sendEmailToOrgAdmin(adminEmail string, admin string, company string, projectNames []string, contributorID string, contributorName string, corporateConsole string) {
	subject := fmt.Sprintf("EasyCLA:  Invitation to Sign the %s Corporate CLA and add to approved list %s ", company, contributorID)
	recipients := []string{adminEmail}
	body := fmt.Sprintf(`
<p>Hello %s,</p>
<p>This is a notification email from EasyCLA regarding the project(s) %s.</p>
<p>The following contributor is requesting to sign CLA for organization: </p>
<p> %s %s </p>
<p>Before the user contribution can be accepted, your organization must sign a CLA.
<p>Kindly login to this portal %s and sign the CLA for any of the projects %s. </p>
<p>Please notify the contributor once they are added so that they may complete the contribution process.</p>
%s
%s`,
		admin, projectNames, contributorName, contributorID, corporateConsole, projectNames,
		utils.GetEmailHelpContent(true), utils.GetEmailSignOffContent())

	err := utils.SendEmail(subject, body, recipients)
	if err != nil {
		log.Warnf("problem sending email with subject: %s to recipients: %+v, error: %+v", subject, recipients, err)
	} else {
		log.Debugf("sent email with subject: %s to recipients: %+v", subject, recipients)
	}
}

func contributorEmailToOrgAdmin(adminEmail string, admin string, company string, projectNames []string, contributor *v1Models.User, corporateConsole string) {
	subject := fmt.Sprintf("EasyCLA:  Invitation to Sign the %s Corporate CLA and add to approved list %s ", company, getBestUserName(contributor))
	recipients := []string{adminEmail}
	body := fmt.Sprintf(`
<p>Hello %s,</p>
<p>This is a notification email from EasyCLA regarding the project(s) %s.</p>
<p>The following contributor is requesting to sign CLA for organization: </p>
<p>%s</p>
<p>Before the user contribution can be accepted, your organization must sign a CLA.
<p>Kindly login to this portal %s and sign the CLA for any of the projects %s. </p>
<p>Please notify the contributor once they are added so that they may complete the contribution process.</p>
%s
%s`,
		admin, projectNames, getFormattedUserDetails(contributor), corporateConsole, projectNames,
		utils.GetEmailHelpContent(true), utils.GetEmailSignOffContent())

	err := utils.SendEmail(subject, body, recipients)
	if err != nil {
		log.Warnf("problem sending email with subject: %s to recipients: %+v, error: %+v", subject, recipients, err)
	} else {
		log.Debugf("sent email with subject: %s to recipients: %+v", subject, recipients)
	}
}

func sendEmailToCLAManagerDesignee(corporateConsole string, companyName string, projectNames []string, designeeEmail string, designeeName string, contributorID string, contributorName string) {
	subject := fmt.Sprintf("EasyCLA:  Invitation to Sign the %s Corporate CLA and add to approved list %s ",
		companyName, contributorID)
	recipients := []string{designeeEmail}
	body := fmt.Sprintf(`
<p>Hello %s,</p>
<p>This is a notification email from EasyCLA regarding the project(s) %s.</p>
<p>The following contributor is requesting to sign CLA for organization: </p>
<p> %s (%s) </p>
<p>Before the user contribution can be accepted, your organization must sign a CLA.
<p>Kindly login to this portal %s and sign the CLA for one of the project(s) %s. </p>
<p>Please notify the contributor once they are added so that they may complete the contribution process.</p>
%s
%s`,
		designeeName, projectNames, contributorID, contributorName, corporateConsole, projectNames,
		utils.GetEmailHelpContent(true), utils.GetEmailSignOffContent())

	err := utils.SendEmail(subject, body, recipients)
	if err != nil {
		log.Warnf("problem sending email with subject: %s to recipients: %+v, error: %+v", subject, recipients, err)
	} else {
		log.Debugf("sent email with subject: %s to recipients: %+v", subject, recipients)
	}
}

// sendEmailToUserWithNoLFID helper function to send email to a given user with no LFID
func sendEmailToUserWithNoLFID(projectName, requesterUsername, requesterEmail, userWithNoLFIDName, userWithNoLFIDEmail, organizationID string, projectID *string, role string) error {
	// subject string, body string, recipients []string
	subject := "EasyCLA: Invitation to create LF Login and complete process of becoming CLA Manager"
	body := fmt.Sprintf(`
<p>Hello %s,</p>
<p>This is a notification email from EasyCLA regarding the Project %s in the EasyCLA system.</p>
<p>User %s (%s) was trying to add you as a CLA Manager for Project %s but was unable to identify your account details in
the EasyCLA system. In order to become a CLA Manager for Project %s, you will need to accept invite below.
Once complete, notify the user %s and they will be able to add you as a CLA Manager.</p>
<p> <a href="USERACCEPTLINK">Accept Invite</a> </p>
%s
%s`,
		userWithNoLFIDName, projectName,
		requesterUsername, requesterEmail, projectName, projectName,
		requesterUsername,
		utils.GetEmailHelpContent(true), utils.GetEmailSignOffContent())

	acsClient := v2AcsService.GetClient()
	automate := false

	acsErr := acsClient.SendUserInvite(&userWithNoLFIDEmail, role, "project|organization", projectID, organizationID, "userinvite", &subject, &body, automate)
	if acsErr != nil {
		return acsErr
	}
	return nil
}

// buildErrorMessage helper function to build an error message
func buildErrorMessage(errPrefix string, claGroupID string, params cla_manager.CreateCLAManagerParams, err error) string {
	return fmt.Sprintf("%s - problem creating new CLA Manager Request using company SFID: %s, project ID: %s, first name: %s, last name: %s, user email: %s, error: %+v",
		errPrefix, params.CompanySFID, claGroupID, *params.Body.FirstName, *params.Body.LastName, *params.Body.UserEmail, err)
}
