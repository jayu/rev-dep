import { authenticate } from "@auth";

export class UserController {
  login() {
    return authenticate();
  }
}
