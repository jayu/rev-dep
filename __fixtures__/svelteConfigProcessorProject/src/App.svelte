<script context="module" lang="ts">
import { fromModule } from "./module-util";
export const version = fromModule;
</script>

<script lang="ts">
  export
    let title = "Hello";

  import { secret } from "./private";
  import { greet } from "./utils";
  import missing from "non-existent-svelte-pkg";

  const message = greet(secret + String(missing));
  void message;
</script>

<main>{title} {version}</main>
