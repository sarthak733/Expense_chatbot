import { createPromiseClient } from "@connectrpc/connect";
import { transport } from "./transport";
import { AuthService } from "../../gen/expense/v1/auth_connect";
import { UserService } from "../../gen/expense/v1/user_connect";
import { ExpenseService } from "../../gen/expense/v1/expense_connect";

export const authClient = createPromiseClient(AuthService, transport);
export const userClient = createPromiseClient(UserService, transport);
export const expenseClient = createPromiseClient(ExpenseService, transport);
