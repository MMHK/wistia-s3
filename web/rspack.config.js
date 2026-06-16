const path = require('path');
const fs = require('fs');
const rspack = require('@rspack/core');
const dotenv = require('dotenv');
const frp = require('mmhk-frp');
const inquirer = require('inquirer');
const http = require('http');

dotenv.config({ path: path.resolve(__dirname, '.env') });

const IS_DEV = process.env.NODE_ENV === 'development';
const VIDEO_NAME = process.env.VIDEO_NAME || 'Demo Video';
const HASH_ID = process.env.HASH_ID || 'testHashId';
const WISTIA_S3_JS_URL = process.env.WISTIA_S3_JS_URL || 'unknown_url';

const FRP_ENDPOINT = process.env.FRP_ENDPOINT || 'localhost';
const FRP_ENDPOINT_PORT = process.env.FRP_ENDPOINT_PORT || 7000;
const FRP_API_PORT = process.env.FRP_API_PORT || 7001;
const FRP_API_USER = process.env.FRP_API_USER || 'admin';
const FRP_API_PWD = process.env.FRP_API_PWD || 'admin';
const FRP_PUBLIC_DOMAIN = process.env.FRP_PUBLIC_DOMAIN || 'localhost';

const checkSubDomainExist = (domain) => {
  const auth = `${FRP_API_USER}:${FRP_API_PWD}`;
  return new Promise((resolve, reject) => {
    const req = http.get({
      hostname: FRP_ENDPOINT,
      port: FRP_API_PORT,
      path: '/api/proxy/http',
      headers: { 'Content-Type': 'application/json' },
      auth: auth,
    }, (resp) => {
      resp.setEncoding('utf8');
      let data = '';
      resp.on('data', (chunk) => { data += chunk; });
      resp.on('end', () => {
        try { resolve(JSON.parse(data)); }
        catch (err) { reject(err); }
      });
    });
    req.on('error', (err) => {
      console.error(`http error: ${err}`);
      reject(err);
    });
    req.end();
  }).then((data) => {
    const list = Array.from(data.proxies || []);
    if (list.find((row) => row.name === domain && row.status === 'online')) {
      return Promise.resolve(true);
    }
    return Promise.resolve(false);
  });
};

class DevTemplatePlugin {
  apply(compiler) {
    compiler.hooks.compilation.tap('DevTemplatePlugin', (compilation) => {
      const hooks = rspack.HtmlRspackPlugin.getCompilationHooks(compilation);
      const templateData = { VideoName: VIDEO_NAME, HashId: HASH_ID, WistiaS3JSUrl: WISTIA_S3_JS_URL };
      hooks.beforeEmit.tapPromise('DevTemplatePlugin', async (data) => {
        data.html = data.html.replace(/\{\{\.(\w+)\}\}/g, (match, key) => templateData[key] || match);
      });
    });
  }
}

class InlineJSPlugin {
  apply(compiler) {
    compiler.hooks.compilation.tap('InlineJSPlugin', (compilation) => {
      const hooks = rspack.HtmlRspackPlugin.getCompilationHooks(compilation);
      hooks.beforeEmit.tapPromise('InlineJSPlugin', async (data) => {
        let html = data.html;

        html = html.replace(/<script[^>]*src=["']([^"']+)["'][^>]*>\s*<\/script>/g, (match, src) => {
          if (src.includes('{{')) return match;
          const asset = compilation.assets[src];
          if (asset) return `<script>${asset.source()}</script>`;
          return match;
        });

        html = html.replace(/<link[^>]*href=["']([^"']+)["'][^>]*\/?>/g, (match, href) => {
          if (href.includes('{{')) return match;
          if (!href.endsWith('.css')) return match;
          const asset = compilation.assets[href];
          if (asset) return `<style>${asset.source()}</style>`;
          return match;
        });

        const faviconPath = path.resolve(__dirname, 'src/favicon.ico');
        if (fs.existsSync(faviconPath)) {
          const b64 = fs.readFileSync(faviconPath).toString('base64');
          html = html.replace(/href=["']favicon\.ico["']/g, `href="data:image/x-icon;base64,${b64}"`);
        }

        data.html = html;
      });
    });
  }
}

const plugins = [
  new rspack.ProgressPlugin(),
  new rspack.HtmlRspackPlugin({
    filename: 'demo.html',
    template: path.resolve(__dirname, 'src/demo.html'),
    inject: 'body',
    excludeChunks: ['wistia-s3'],
    minify: !IS_DEV,
  }),
  new rspack.HtmlRspackPlugin({
    filename: 'index.html',
    template: path.resolve(__dirname, 'src/index.html'),
    inject: 'body',
    excludeChunks: ['demo', 'wistia-s3'],
    minify: !IS_DEV,
  }),
  new rspack.CopyRspackPlugin({
    patterns: [{ from: 'src/favicon.ico', to: 'favicon.ico' }],
  }),
];

if (IS_DEV) {
  plugins.push(new DevTemplatePlugin());
} else {
  plugins.push(new InlineJSPlugin());
}

const baseConfig = {
  mode: IS_DEV ? 'development' : 'production',
  experiments: { css: true },
  entry: {
    'wistia-s3': ['./src/main.js'],
    'demo': ['./src/demo.js'],
  },
  output: {
    filename: IS_DEV ? '[name].js' : '[name].min.js',
    path: path.resolve(__dirname, 'dist'),
    publicPath: 'auto',
    clean: true,
  },
  plugins,
  module: {
    rules: [
      {
        test: /\.css$/,
        type: 'css',
      },
      {
        test: /\.(png|jpe?g|gif|svg|webp|ico|eot|ttf|otf|woff2?)$/i,
        type: 'asset',
        generator: { filename: 'assets/[hash][ext][query]' },
        parser: { dataUrlCondition: { maxSize: 200 * 1024 } },
      },
    ],
  },
  optimization: {
    splitChunks: false,
    runtimeChunk: false,
    minimize: !IS_DEV,
  },
  performance: { hints: false },
  devtool: 'source-map',
  watchOptions: { ignored: /(node_modules)/ },
  devServer: {
    open: true,
    compress: true,
    hot: true,
    static: { directory: path.join(__dirname, 'dist') },
    allowedHosts: ['localhost', '.demo2.mixmedia.com'],
  },
};

if (!IS_DEV) {
  module.exports = baseConfig;
} else {
  const prompt = inquirer.createPromptModule();
  module.exports = prompt([
    {
      type: 'list',
      name: 'public',
      message: '是否允許外網訪問',
      choices: [
        { name: '允許', value: true },
        { name: '不需要', value: false },
      ],
    },
    {
      type: 'input',
      name: 'subdomain',
      message: '請配一個域名',
      validate: (input) => /^([a-z0-9\-]{4,})$/i.test(input),
      when: ({ public }) => public,
    },
  ]).then(({ public: usePublic, subdomain }) => {
    if (usePublic && subdomain) {
      baseConfig.devServer = {
        ...baseConfig.devServer,
        client: {
          webSocketURL: `https://${subdomain}.${FRP_PUBLIC_DOMAIN}/ws`,
        },
        open: {
          target: `https://${subdomain}.${FRP_PUBLIC_DOMAIN}`,
        },
        allowedHosts: [`.${FRP_PUBLIC_DOMAIN}`],
        onListening: (devServer) => {
          if (!devServer) return;
          const addr = devServer.server.address();
          console.log('set domain:', subdomain);

          checkSubDomainExist(subdomain)
            .then((exist) => {
              if (!exist) {
                const conf = {
                  common: {
                    serverPort: FRP_ENDPOINT_PORT,
                    serverAddr: FRP_ENDPOINT,
                  },
                };
                conf[subdomain] = {
                  type: 'http',
                  localIp: '127.0.0.1',
                  localPort: addr.port,
                  subdomain,
                };
                return frp.startClient(conf);
              }
              return Promise.reject(new Error('域名已被使用'));
            })
            .catch((err) => {
              console.error(err);
              return Promise.reject(err);
            });
        },
      };
    }
    return baseConfig;
  });
}
