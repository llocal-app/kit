import { createRoot } from "react-dom/client";
import { Admin, EditGuesser, ListGuesser, Resource } from "react-admin";
import { dataProvider } from "./dataProvider";

createRoot(document.getElementById("root")!).render(
  <Admin dataProvider={dataProvider}>
    <Resource name="vendors" list={ListGuesser} edit={EditGuesser} />
    <Resource name="purchase_orders" list={ListGuesser} edit={EditGuesser} />
  </Admin>,
);
