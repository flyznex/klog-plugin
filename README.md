# KLog-Plugin
 - Krakend plugin for logging request

Now load plugin in the configuration.

```
  "plugin": {
    "pattern": ".so",
    "folder": "./plugins/"
  },
```
Add the plugin and `extra_config` entries in your configuration
```
"plugin/http-server": {
    "name": [
       "klog-plugin"
    ],
    "klog-plugin": {
        "skip_paths": ["/path-skip-logging","/__health"],
        "enabled": true
    }
}
```