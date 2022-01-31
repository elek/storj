// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

const path = require('path');

module.exports = {
    // vue.config.js

    devServer: {
        proxy: {
            "/api": {
                target: "https://localhost:7777",
                secure: false
            }
        }
    },
    publicPath: '/static/dist',
    productionSourceMap: false,
    parallel: true,
    lintOnSave: false, // disables eslint for builds
    configureWebpack: {
        plugins: [],
    },
    chainWebpack: config => {
        config.output.chunkFilename('js/vendors_[hash].js');
        config.output.filename('js/app_[hash].js');

        config.resolve.alias
            .set('@', path.resolve('src'));

        config
            .plugin('html')
            .tap(args => {
                args[0].template = './index.html';

                return args;
            });

        const svgRule = config.module.rule('svg');

        svgRule.uses.clear();

        svgRule
            .use('babel-loader')
            .loader('babel-loader')
            .end()
            .use('vue-svg-loader')
            .loader('vue-svg-loader');
    },
};
