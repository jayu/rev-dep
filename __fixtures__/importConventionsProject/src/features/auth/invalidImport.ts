import { authenticate } from "@auth/utils";

export class AuthService {
  login() {
    return authenticate();
  }
}
