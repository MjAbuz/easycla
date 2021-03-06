// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

import { Component } from '@angular/core';
import { NavController, NavParams, ViewController, ModalController, IonicPage } from 'ionic-angular';
import { ClaService } from '../../services/cla.service';
import { AuthService } from '../../services/auth.service';
import { Restricted } from '../../decorators/restricted';
import { generalConstants } from '../../constants/general';

@Restricted({
  roles: ['isAuthenticated']
})
@IonicPage({
  segment: 'cla/gerrit/project/:projectId/corporate'
})
@Component({
  selector: 'cla-gerrit-corporate',
  templateUrl: 'cla-gerrit-corporate.html',
  providers: []
})
export class ClaGerritCorporatePage {
  loading: any;
  projectId: string;
  userId: string;
  signature: string;
  companies: any;
  filteredCompanies: any;
  expanded: boolean = true;
  errorMessage: string = null;
  searchTimer = null;

  constructor(
    public navCtrl: NavController,
    public navParams: NavParams,
    public viewCtrl: ViewController,
    private modalCtrl: ModalController,
    private claService: ClaService,
    private authService: AuthService,
  ) {
    this.projectId = navParams.get('projectId');
    this.getDefaults();
    localStorage.setItem('projectId', this.projectId);
    localStorage.setItem('gerritClaType', 'CCLA');
  }

  getDefaults() {
    this.loading = {
      companies: true
    };
    this.companies = [];
    this.filteredCompanies = [];
  }

  ngOnInit() {
    this.authService.userProfile$.subscribe(user => {
      if (user !== undefined) {
        if (user) {
          this.getProject();
        } else {
          this.redirectToLogin();
        }
      }
    });
  }

  redirectToLogin() {
    this.navCtrl.setRoot('LoginPage');
  }

  getCompanies() {
    this.claService.getAllCompanies().subscribe((response) => {
      if (response) {
        this.companies = response;
        this.filteredCompanies = this.companies;
      }
      this.loading.companies = false;
    });
  }

  getUserInfo() {
    // retrieve userInfo from auth0 service
    this.claService.postOrGetUserForGerrit().subscribe((user) => {
      localStorage.setItem(generalConstants.USER_MODEL, JSON.stringify(user));
      this.userId = user.user_id;
      this.getCompanies();
    }, (error) => {
      // Got an auth error, redirect to the login
      this.loading = false;
      setTimeout(() => this.redirectToLogin());
    });
  }

  openClaEmployeeCompanyConfirmPage(company) {
    let data = {
      project_id: this.projectId,
      company_id: company.company_id,
      user_id: this.userId
    };
    this.claService.postCheckAndPreparedEmployeeSignature(data).subscribe((response) => {
      let errors = response.hasOwnProperty('errors');
      if (errors) {
        if (response.errors.hasOwnProperty('missing_ccla')) {
          // When the company does NOT have a CCLA with the project: {'errors': {'missing_ccla': 'Company does not have CCLA with this project'}}
          this.openClaSendClaManagerEmailModal(company);
        }

        if (response.errors.hasOwnProperty('ccla_approval_list')) {
          // When the user is not whitelisted with the company: return {'errors': {'ccla_approval_list': 'No user email authorized for this ccla'}}
          this.openClaEmployeeCompanyTroubleshootPage(company);
          return;
        }
      } else {
        this.signature = response;

        this.navCtrl.push('ClaEmployeeCompanyConfirmPage', {
          projectId: this.projectId,
          signingType: 'Gerrit',
          userId: this.userId,
          companyId: company.company_id
        });
      }
    });
  }

  openClaSendClaManagerEmailModal(company) {
    let modal = this.modalCtrl.create('ClaSendClaManagerEmailModal', {
      projectId: this.projectId,
      userId: this.userId,
      companyId: company.company_id,
      authenticated: true
    });
    modal.present();
  }

  openClaNewCompanyModal() {
    let modal = this.modalCtrl.create('ClaNewCompanyModal', {
      projectId: this.projectId
    });
    modal.present();
  }

  openClaCompanyAdminYesnoModal() {
    let modal = this.modalCtrl.create('ClaCompanyAdminYesnoModal', {
      projectId: this.projectId,
      userId: this.userId,
      authenticated: true
    });
    modal.present();
  }

  openClaEmployeeCompanyTroubleshootPage(company) {
    this.navCtrl.push('ClaEmployeeCompanyTroubleshootPage', {
      projectId: this.projectId,
      repositoryId: '',
      userId: this.userId,
      companyId: company.company_id,
      gitService: 'Gerrit'
    });
  }

  getProject() {
    this.claService.getProjectWithAuthToken(this.projectId).subscribe(
      (project) => {
        this.errorMessage = '';
        localStorage.setItem(generalConstants.PROJECT_MODEL, JSON.stringify(project));
        this.getUserInfo();
      },
      () => {
        this.loading = false;
        this.errorMessage = 'Invalid project id.';
      }
    );
  }

  onSearch(event) {
    const searchText = event._value;
    if (this.searchTimer !== null) {
      clearTimeout(this.searchTimer);
    }
    this.searchTimer = setTimeout(() => {
      if (searchText === '') {
        this.filteredCompanies = this.companies;
      } else {
        this.filteredCompanies = this.companies.filter((a) => {
          return a.company_name.toLowerCase().includes(searchText.toLowerCase());
        });
      }
    }, 250);
  }

  onClickToggle(hasExpanded) {
    this.expanded = hasExpanded;
  }
}
