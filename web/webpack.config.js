const path = require("path");
const fs = require('fs');
const webpack = require("webpack");
const dotenv = require('dotenv');
dotenv.config().parsed;


const IS_DEVSERVER = process.env.WEBPACK_DEV_SERVER || process.env.WEBPACK_SERVE;
const VIDEO_NAME = process.env.VIDEO_NAME || 'Demo Video';
const HASH_ID = process.env.HASH_ID || 'testHashId';
const WISTIA_S3_JS_URL = process.env.WISTIA_S3_JS_URL || 'unknown_url';

/*
 * SplitChunksPlugin is enabled by default and replaced
 * deprecated CommonsChunkPlugin. It automatically identifies modules which
 * should be splitted of chunk by heuristics using module duplication count and
 * module category (i. e. node_modules). And splits the chunks…
 *
 * It is safe to remove "splitChunks" from the generated configuration
 * and was added as an educational example.
 *
 * https://webpack.js.org/plugins/split-chunks-plugin/
 *
 */

const { CleanWebpackPlugin } = require('clean-webpack-plugin');
const TerserWebpackPlugin = require('terser-webpack-plugin');
const HtmlWebpackPlugin = require('html-webpack-plugin');
const HtmlWebpackExcludeAssetsPlugin = require('html-webpack-exclude-assets-plugin-webpack5');
const HtmlInlineScriptPlugin = require('html-inline-script-webpack-plugin');
const HTMLInlineCSSWebpackPlugin = require('html-inline-css-webpack-plugin').default;
const MiniCssExtractPlugin = require('mini-css-extract-plugin');

/*
 * We've enabled HtmlWebpackPlugin for you! This generates a html
 * page for you when you compile webpack, which will make you start
 * developing and prototyping faster.
 *
 * https://github.com/jantimon/html-webpack-plugin
 *
 */



const config = {
	mode: IS_DEVSERVER ? 'development' : 'production',
	entry: {
    "wistia-s3": ['./src/main.js'],
    "demo": ["./src/demo.js"],
  },

	output: {
	  filename: IS_DEVSERVER ? '[name].js' : '[name].min.js',
		path: path.resolve(__dirname, 'dist'),
		publicPath: 'auto',
		clean: true,

	},

	plugins: [
		new webpack.ProgressPlugin(),
		new CleanWebpackPlugin(),
    new HtmlWebpackPlugin({
      filename: "demo.html",
      template: path.relative(__dirname, "src/demo.html"),
      templateContent: !IS_DEVSERVER ? false :({ htmlWebpackPlugin }) => {
        let template = fs.readFileSync(path.resolve(__dirname, 'src/demo.html'), 'utf8');
        return template.replace(/\{\{\.([^\}\.]+)\}\}/g, (match, p1) => {
          return htmlWebpackPlugin.options.templateParameters[p1] | '';
        });
      },
      // hash: true,
      inject: "body",
      excludeAssets: [/wistia-s3.min.js/],
      templateParameters: {
        VideoName: VIDEO_NAME,
        HashId: HASH_ID,
        WistiaS3JSUrl: WISTIA_S3_JS_URL,
      },
    }),
    new HtmlWebpackPlugin({
      filename: "index.html",
      template: path.relative(__dirname, "src/index.html"),
      templateContent: !IS_DEVSERVER ? false :({ htmlWebpackPlugin }) => {
        let template = fs.readFileSync(path.resolve(__dirname, 'src/index.html'), 'utf8');
        return template.replace(/\{\{\.([^\}\.]+)\}\}/g, (match, p1) => {
          return htmlWebpackPlugin.options.templateParameters[p1] || '';
        });
      },
      // hash: true,
      inject: "body",
      excludeAssets: [/wistia-s3.min.js/, /wistia-s3.js/, /demo.js/, /demo.min.js/],
      templateParameters: {
        VideoName: VIDEO_NAME,
        HashId: HASH_ID,
        WistiaS3JSUrl: WISTIA_S3_JS_URL,
      },
    }),
    new MiniCssExtractPlugin({
      // Options similar to the same options in webpackOptions.output
      // all options are optional
      filename: 'css/[name].css',
      chunkFilename: 'css/[id].css',
      ignoreOrder: false, // Enable to remove warnings about conflicting order
    }),

  ].concat(IS_DEVSERVER ? [
    new HtmlWebpackExcludeAssetsPlugin(),
  ]: [
    new HTMLInlineCSSWebpackPlugin(),
    new HtmlInlineScriptPlugin({
      htmlMatchPattern: [/demo.html$/],
      scriptMatchPattern: [/demo.min.js$/],
    }),
    new HtmlWebpackExcludeAssetsPlugin(),
  ]),

	module: {
		rules: [
      {
        test: /\.html$/,
        use: [
          {
            loader: 'html-loader',
          },
        ]
      },
      {
        test: /\.css$/,
        use: [
          MiniCssExtractPlugin.loader,
          {
            loader: 'css-loader',
          },
        ]
      },
			{
				test: /.(js)$/,
				include: [
					path.resolve(__dirname, 'src'),
				],
				exclude: /(node_modules|webpack)/,
				use: [
					{
						loader: 'babel-loader',
						options: {
							plugins: [
								[
									"@babel/plugin-transform-template-literals", {
									loose: true
								}],
								"@babel/plugin-transform-runtime",
								"@babel/plugin-syntax-dynamic-import",
                "dynamic-import-node"
							],

							presets: [
								[
									'@babel/preset-env',
									{
										modules: false,
										useBuiltIns: "usage",
										corejs: 3
									}
								]
							]
						}
					},
				],
			},
      {
        test: /\.(eot|svg|ttf|woff|woff2|ico|png|gif|jpeg|jpg)$/i,
        type: 'asset',
        generator: {
          filename: 'assets/[hash][ext][query]'
        },
        parser: {
          dataUrlCondition: {
            maxSize: 200 * 1024 // 200kb
          }
        },
      },
		]
	},

	optimization: {
    splitChunks: IS_DEVSERVER ? {} : false,
    runtimeChunk: IS_DEVSERVER ? {} : false,
    minimizer: [
			new TerserWebpackPlugin({
				extractComments: false,
				terserOptions: {
					format: {
						comments: false,
					},
					compress: {
						drop_console: true, // 移除 console.log 语句
					},
				},
			}),
		],

		minimize: !IS_DEVSERVER,
	},

	devtool: "source-map",
	watchOptions: {
		ignored: /(node_modules|webpack)/
	},
	devServer: {
		open: true,
		compress: true,
    hot: !!(IS_DEVSERVER),
		static: {
			directory: path.join(__dirname, 'dist'),
		},
		allowedHosts: [
			"localhost",
			".demo2.mixmedia.com",
		],
	}
};

module.exports = config;

