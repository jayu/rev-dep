import { authenticate } from "./utils";

export class AuthService {
  login() {
    return authenticate();
  }
}
