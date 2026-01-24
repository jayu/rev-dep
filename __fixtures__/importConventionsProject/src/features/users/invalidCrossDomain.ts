import { authenticate } from "../auth/utils";

export class UserController {
  login() {
    return authenticate();
  }
}
