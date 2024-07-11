const path = require("path");
const webpack = require("webpack");

const IS_DEVSERVER = process.env.WEBPACK_DEV_SERVER || process.env.WEBPACK_SERVE;


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
    "wistia-s3": ['./src/main.js']
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
    new webpack.DefinePlugin({
      __MEDIA_ENDPOINT: JSON.stringify(process.env.MEDIA_ENDPOINT),
    })
	],

	module: {
		rules: [
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
								"@babel/plugin-syntax-dynamic-import"
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
		]
	},

	optimization: {
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

