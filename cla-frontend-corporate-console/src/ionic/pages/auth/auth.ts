// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

import { Component, OnInit } from '@angular/core';
import { NavController } from 'ionic-angular';
import { AuthService } from '../../services/auth.service';
import { EnvConfig } from '../../services/cla.env.utils';

/**
 * Generated class for the AuthPage page.
 *
 * See https://ionicframework.com/docs/components/#navigation for more info on
 * Ionic pages and navigation.
 */
@Component({
  selector: 'page-auth',
  templateUrl: 'auth.html'
})
export class AuthPage implements OnInit {
  userRoles: any;

  constructor(
    public navCtrl: NavController,
    public authService: AuthService,
  ) { }

  ngOnInit() {
    this.authService.redirectRoot.subscribe((target) => {
      if (EnvConfig['lfx-header-enabled'] === "true") {
        this.navCtrl.setRoot('CompaniesPage');
      } else {
        // Redirected forcefully for without LFX-header due to auth token persist in URL. 
        window.open(target, '_self');
      }
    });
  }
}
