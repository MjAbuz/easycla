// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

import { Injectable } from '@angular/core';

@Injectable()
export class AppSettings {
    /* eslint-disable */
    public static ID_TOKEN = 'id_token';
    public static ACCESS_TOKEN = 'access_token';
    public static EXPIRES_AT = 'expires_at';
    public static USER_ID = 'userid';
    public static USER_EMAIL = 'user_email';
    public static USER_NAME = 'user_name';
    public static CONSOLE_TYPE = 'consoleType';
    public static PROJECT_CONSOLE_LINK = 'proj-console-link';
    public static CORPORATE_CONSOLE_LINK = 'corp-console-link';
    public static LEARN_MORE = 'https://docs.linuxfoundation.org/docs/communitybridge/easycla';
    public static TICKET_URL = 'https://jira.linuxfoundation.org/servicedesk/customer/portal/4/create/143';
}
