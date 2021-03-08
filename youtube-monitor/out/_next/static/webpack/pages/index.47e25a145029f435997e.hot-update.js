webpackHotUpdate_N_E("pages/index",{

/***/ "./components/Message.tsx":
/*!********************************!*\
  !*** ./components/Message.tsx ***!
  \********************************/
/*! exports provided: default */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
__webpack_require__.r(__webpack_exports__);
/* WEBPACK VAR INJECTION */(function(module) {/* harmony import */ var react_jsx_dev_runtime__WEBPACK_IMPORTED_MODULE_0__ = __webpack_require__(/*! react/jsx-dev-runtime */ "./node_modules/react/jsx-dev-runtime.js");
/* harmony import */ var react_jsx_dev_runtime__WEBPACK_IMPORTED_MODULE_0___default = /*#__PURE__*/__webpack_require__.n(react_jsx_dev_runtime__WEBPACK_IMPORTED_MODULE_0__);
/* harmony import */ var C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_classCallCheck__WEBPACK_IMPORTED_MODULE_1__ = __webpack_require__(/*! ./node_modules/@babel/runtime/helpers/esm/classCallCheck */ "./node_modules/@babel/runtime/helpers/esm/classCallCheck.js");
/* harmony import */ var C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_createClass__WEBPACK_IMPORTED_MODULE_2__ = __webpack_require__(/*! ./node_modules/@babel/runtime/helpers/esm/createClass */ "./node_modules/@babel/runtime/helpers/esm/createClass.js");
/* harmony import */ var C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_inherits__WEBPACK_IMPORTED_MODULE_3__ = __webpack_require__(/*! ./node_modules/@babel/runtime/helpers/esm/inherits */ "./node_modules/@babel/runtime/helpers/esm/inherits.js");
/* harmony import */ var C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_possibleConstructorReturn__WEBPACK_IMPORTED_MODULE_4__ = __webpack_require__(/*! ./node_modules/@babel/runtime/helpers/esm/possibleConstructorReturn */ "./node_modules/@babel/runtime/helpers/esm/possibleConstructorReturn.js");
/* harmony import */ var C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_getPrototypeOf__WEBPACK_IMPORTED_MODULE_5__ = __webpack_require__(/*! ./node_modules/@babel/runtime/helpers/esm/getPrototypeOf */ "./node_modules/@babel/runtime/helpers/esm/getPrototypeOf.js");
/* harmony import */ var react__WEBPACK_IMPORTED_MODULE_6__ = __webpack_require__(/*! react */ "./node_modules/react/index.js");
/* harmony import */ var react__WEBPACK_IMPORTED_MODULE_6___default = /*#__PURE__*/__webpack_require__.n(react__WEBPACK_IMPORTED_MODULE_6__);
/* harmony import */ var _Message_module_sass__WEBPACK_IMPORTED_MODULE_7__ = __webpack_require__(/*! ./Message.module.sass */ "./components/Message.module.sass");
/* harmony import */ var _Message_module_sass__WEBPACK_IMPORTED_MODULE_7___default = /*#__PURE__*/__webpack_require__.n(_Message_module_sass__WEBPACK_IMPORTED_MODULE_7__);






var _jsxFileName = "C:\\Users\\momom\\Documents\\GitHub\\youtube-study-space\\youtube-monitor\\components\\Message.tsx";

function _createSuper(Derived) { var hasNativeReflectConstruct = _isNativeReflectConstruct(); return function _createSuperInternal() { var Super = Object(C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_getPrototypeOf__WEBPACK_IMPORTED_MODULE_5__["default"])(Derived), result; if (hasNativeReflectConstruct) { var NewTarget = Object(C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_getPrototypeOf__WEBPACK_IMPORTED_MODULE_5__["default"])(this).constructor; result = Reflect.construct(Super, arguments, NewTarget); } else { result = Super.apply(this, arguments); } return Object(C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_possibleConstructorReturn__WEBPACK_IMPORTED_MODULE_4__["default"])(this, result); }; }

function _isNativeReflectConstruct() { if (typeof Reflect === "undefined" || !Reflect.construct) return false; if (Reflect.construct.sham) return false; if (typeof Proxy === "function") return true; try { Date.prototype.toString.call(Reflect.construct(Date, [], function () {})); return true; } catch (e) { return false; } }




var Message = /*#__PURE__*/function (_React$Component) {
  Object(C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_inherits__WEBPACK_IMPORTED_MODULE_3__["default"])(Message, _React$Component);

  var _super = _createSuper(Message);

  function Message() {
    Object(C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_classCallCheck__WEBPACK_IMPORTED_MODULE_1__["default"])(this, Message);

    return _super.apply(this, arguments);
  }

  Object(C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_createClass__WEBPACK_IMPORTED_MODULE_2__["default"])(Message, [{
    key: "render",
    value: function render() {
      if (this.props.default_room_state && this.props.no_seat_room_state) {
        var numWorkers = this.props.default_room_state.seats.length + this.props.no_seat_room_state.seats.length;
        return /*#__PURE__*/Object(react_jsx_dev_runtime__WEBPACK_IMPORTED_MODULE_0__["jsxDEV"])("div", {
          id: _Message_module_sass__WEBPACK_IMPORTED_MODULE_7___default.a.message,
          children: ["Currently ", numWorkers, " people working! \uD83D\uDD25"]
        }, void 0, true, {
          fileName: _jsxFileName,
          lineNumber: 15,
          columnNumber: 9
        }, this); // todo return <div id={styles.message}>現在、{numWorkers}人が作業中🔥</div>;
      } else {
        return /*#__PURE__*/Object(react_jsx_dev_runtime__WEBPACK_IMPORTED_MODULE_0__["jsxDEV"])("div", {
          id: _Message_module_sass__WEBPACK_IMPORTED_MODULE_7___default.a.message
        }, void 0, false, {
          fileName: _jsxFileName,
          lineNumber: 19,
          columnNumber: 14
        }, this);
      }
    }
  }]);

  return Message;
}(react__WEBPACK_IMPORTED_MODULE_6___default.a.Component);

/* harmony default export */ __webpack_exports__["default"] = (Message);

;
    var _a, _b;
    // Legacy CSS implementations will `eval` browser code in a Node.js context
    // to extract CSS. For backwards compatibility, we need to check we're in a
    // browser context before continuing.
    if (typeof self !== 'undefined' &&
        // AMP / No-JS mode does not inject these helpers:
        '$RefreshHelpers$' in self) {
        var currentExports = module.__proto__.exports;
        var prevExports = (_b = (_a = module.hot.data) === null || _a === void 0 ? void 0 : _a.prevExports) !== null && _b !== void 0 ? _b : null;
        // This cannot happen in MainTemplate because the exports mismatch between
        // templating and execution.
        self.$RefreshHelpers$.registerExportsForReactRefresh(currentExports, module.i);
        // A module can be accepted automatically based on its exports, e.g. when
        // it is a Refresh Boundary.
        if (self.$RefreshHelpers$.isReactRefreshBoundary(currentExports)) {
            // Save the previous exports on update so we can compare the boundary
            // signatures.
            module.hot.dispose(function (data) {
                data.prevExports = currentExports;
            });
            // Unconditionally accept an update to this module, we'll check if it's
            // still a Refresh Boundary later.
            module.hot.accept();
            // This field is set when the previous version of this module was a
            // Refresh Boundary, letting us know we need to check for invalidation or
            // enqueue an update.
            if (prevExports !== null) {
                // A boundary can become ineligible if its exports are incompatible
                // with the previous exports.
                //
                // For example, if you add/remove/change exports, we'll want to
                // re-execute the importing modules, and force those components to
                // re-render. Similarly, if you convert a class component to a
                // function, we want to invalidate the boundary.
                if (self.$RefreshHelpers$.shouldInvalidateReactRefreshBoundary(prevExports, currentExports)) {
                    module.hot.invalidate();
                }
                else {
                    self.$RefreshHelpers$.scheduleUpdate();
                }
            }
        }
        else {
            // Since we just executed the code for the module, it's possible that the
            // new exports made it ineligible for being a boundary.
            // We only care about the case when we were _previously_ a boundary,
            // because we already accepted this update (accidental side effect).
            var isNoLongerABoundary = prevExports !== null;
            if (isNoLongerABoundary) {
                module.hot.invalidate();
            }
        }
    }

/* WEBPACK VAR INJECTION */}.call(this, __webpack_require__(/*! ./../node_modules/next/dist/compiled/webpack/harmony-module.js */ "./node_modules/next/dist/compiled/webpack/harmony-module.js")(module)))

/***/ }),

/***/ "./components/StandingRoom.tsx":
/*!*************************************!*\
  !*** ./components/StandingRoom.tsx ***!
  \*************************************/
/*! exports provided: default */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
__webpack_require__.r(__webpack_exports__);
/* WEBPACK VAR INJECTION */(function(module) {/* harmony import */ var react_jsx_dev_runtime__WEBPACK_IMPORTED_MODULE_0__ = __webpack_require__(/*! react/jsx-dev-runtime */ "./node_modules/react/jsx-dev-runtime.js");
/* harmony import */ var react_jsx_dev_runtime__WEBPACK_IMPORTED_MODULE_0___default = /*#__PURE__*/__webpack_require__.n(react_jsx_dev_runtime__WEBPACK_IMPORTED_MODULE_0__);
/* harmony import */ var C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_classCallCheck__WEBPACK_IMPORTED_MODULE_1__ = __webpack_require__(/*! ./node_modules/@babel/runtime/helpers/esm/classCallCheck */ "./node_modules/@babel/runtime/helpers/esm/classCallCheck.js");
/* harmony import */ var C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_createClass__WEBPACK_IMPORTED_MODULE_2__ = __webpack_require__(/*! ./node_modules/@babel/runtime/helpers/esm/createClass */ "./node_modules/@babel/runtime/helpers/esm/createClass.js");
/* harmony import */ var C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_inherits__WEBPACK_IMPORTED_MODULE_3__ = __webpack_require__(/*! ./node_modules/@babel/runtime/helpers/esm/inherits */ "./node_modules/@babel/runtime/helpers/esm/inherits.js");
/* harmony import */ var C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_possibleConstructorReturn__WEBPACK_IMPORTED_MODULE_4__ = __webpack_require__(/*! ./node_modules/@babel/runtime/helpers/esm/possibleConstructorReturn */ "./node_modules/@babel/runtime/helpers/esm/possibleConstructorReturn.js");
/* harmony import */ var C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_getPrototypeOf__WEBPACK_IMPORTED_MODULE_5__ = __webpack_require__(/*! ./node_modules/@babel/runtime/helpers/esm/getPrototypeOf */ "./node_modules/@babel/runtime/helpers/esm/getPrototypeOf.js");
/* harmony import */ var react__WEBPACK_IMPORTED_MODULE_6__ = __webpack_require__(/*! react */ "./node_modules/react/index.js");
/* harmony import */ var react__WEBPACK_IMPORTED_MODULE_6___default = /*#__PURE__*/__webpack_require__.n(react__WEBPACK_IMPORTED_MODULE_6__);
/* harmony import */ var _StandingRoom_module_sass__WEBPACK_IMPORTED_MODULE_7__ = __webpack_require__(/*! ./StandingRoom.module.sass */ "./components/StandingRoom.module.sass");
/* harmony import */ var _StandingRoom_module_sass__WEBPACK_IMPORTED_MODULE_7___default = /*#__PURE__*/__webpack_require__.n(_StandingRoom_module_sass__WEBPACK_IMPORTED_MODULE_7__);






var _jsxFileName = "C:\\Users\\momom\\Documents\\GitHub\\youtube-study-space\\youtube-monitor\\components\\StandingRoom.tsx";

function _createSuper(Derived) { var hasNativeReflectConstruct = _isNativeReflectConstruct(); return function _createSuperInternal() { var Super = Object(C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_getPrototypeOf__WEBPACK_IMPORTED_MODULE_5__["default"])(Derived), result; if (hasNativeReflectConstruct) { var NewTarget = Object(C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_getPrototypeOf__WEBPACK_IMPORTED_MODULE_5__["default"])(this).constructor; result = Reflect.construct(Super, arguments, NewTarget); } else { result = Super.apply(this, arguments); } return Object(C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_possibleConstructorReturn__WEBPACK_IMPORTED_MODULE_4__["default"])(this, result); }; }

function _isNativeReflectConstruct() { if (typeof Reflect === "undefined" || !Reflect.construct) return false; if (Reflect.construct.sham) return false; if (typeof Proxy === "function") return true; try { Date.prototype.toString.call(Reflect.construct(Date, [], function () {})); return true; } catch (e) { return false; } }




var StandingRoom = /*#__PURE__*/function (_React$Component) {
  Object(C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_inherits__WEBPACK_IMPORTED_MODULE_3__["default"])(StandingRoom, _React$Component);

  var _super = _createSuper(StandingRoom);

  function StandingRoom() {
    Object(C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_classCallCheck__WEBPACK_IMPORTED_MODULE_1__["default"])(this, StandingRoom);

    return _super.apply(this, arguments);
  }

  Object(C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_createClass__WEBPACK_IMPORTED_MODULE_2__["default"])(StandingRoom, [{
    key: "render",
    value: function render() {
      if (this.props.no_seat_room_state) {
        var numStandingWorkers = this.props.no_seat_room_state.seats.length;
        return /*#__PURE__*/Object(react_jsx_dev_runtime__WEBPACK_IMPORTED_MODULE_0__["jsxDEV"])("div", {
          id: _StandingRoom_module_sass__WEBPACK_IMPORTED_MODULE_7___default.a.standingRoom,
          children: [/*#__PURE__*/Object(react_jsx_dev_runtime__WEBPACK_IMPORTED_MODULE_0__["jsxDEV"])("h2", {
            children: "\u3000"
          }, void 0, false, {
            fileName: _jsxFileName,
            lineNumber: 14,
            columnNumber: 11
          }, this), /*#__PURE__*/Object(react_jsx_dev_runtime__WEBPACK_IMPORTED_MODULE_0__["jsxDEV"])("h2", {
            children: "Standing Room"
          }, void 0, false, {
            fileName: _jsxFileName,
            lineNumber: 15,
            columnNumber: 11
          }, this), /*#__PURE__*/Object(react_jsx_dev_runtime__WEBPACK_IMPORTED_MODULE_0__["jsxDEV"])("h3", {
            children: ["\uFF08", /*#__PURE__*/Object(react_jsx_dev_runtime__WEBPACK_IMPORTED_MODULE_0__["jsxDEV"])("span", {
              className: _StandingRoom_module_sass__WEBPACK_IMPORTED_MODULE_7___default.a.commandString,
              children: "!0"
            }, void 0, false, {
              fileName: _jsxFileName,
              lineNumber: 18,
              columnNumber: 14
            }, this), " \u3067\u5165\u5BA4\uFF09"]
          }, void 0, true, {
            fileName: _jsxFileName,
            lineNumber: 17,
            columnNumber: 11
          }, this), /*#__PURE__*/Object(react_jsx_dev_runtime__WEBPACK_IMPORTED_MODULE_0__["jsxDEV"])("h2", {
            children: [numStandingWorkers, "\u4EBA"]
          }, void 0, true, {
            fileName: _jsxFileName,
            lineNumber: 20,
            columnNumber: 11
          }, this)]
        }, void 0, true, {
          fileName: _jsxFileName,
          lineNumber: 13,
          columnNumber: 9
        }, this);
      } else {
        return /*#__PURE__*/Object(react_jsx_dev_runtime__WEBPACK_IMPORTED_MODULE_0__["jsxDEV"])("div", {
          id: _StandingRoom_module_sass__WEBPACK_IMPORTED_MODULE_7___default.a.standingRoom
        }, void 0, false, {
          fileName: _jsxFileName,
          lineNumber: 24,
          columnNumber: 14
        }, this);
      }
    }
  }]);

  return StandingRoom;
}(react__WEBPACK_IMPORTED_MODULE_6___default.a.Component);

/* harmony default export */ __webpack_exports__["default"] = (StandingRoom);

;
    var _a, _b;
    // Legacy CSS implementations will `eval` browser code in a Node.js context
    // to extract CSS. For backwards compatibility, we need to check we're in a
    // browser context before continuing.
    if (typeof self !== 'undefined' &&
        // AMP / No-JS mode does not inject these helpers:
        '$RefreshHelpers$' in self) {
        var currentExports = module.__proto__.exports;
        var prevExports = (_b = (_a = module.hot.data) === null || _a === void 0 ? void 0 : _a.prevExports) !== null && _b !== void 0 ? _b : null;
        // This cannot happen in MainTemplate because the exports mismatch between
        // templating and execution.
        self.$RefreshHelpers$.registerExportsForReactRefresh(currentExports, module.i);
        // A module can be accepted automatically based on its exports, e.g. when
        // it is a Refresh Boundary.
        if (self.$RefreshHelpers$.isReactRefreshBoundary(currentExports)) {
            // Save the previous exports on update so we can compare the boundary
            // signatures.
            module.hot.dispose(function (data) {
                data.prevExports = currentExports;
            });
            // Unconditionally accept an update to this module, we'll check if it's
            // still a Refresh Boundary later.
            module.hot.accept();
            // This field is set when the previous version of this module was a
            // Refresh Boundary, letting us know we need to check for invalidation or
            // enqueue an update.
            if (prevExports !== null) {
                // A boundary can become ineligible if its exports are incompatible
                // with the previous exports.
                //
                // For example, if you add/remove/change exports, we'll want to
                // re-execute the importing modules, and force those components to
                // re-render. Similarly, if you convert a class component to a
                // function, we want to invalidate the boundary.
                if (self.$RefreshHelpers$.shouldInvalidateReactRefreshBoundary(prevExports, currentExports)) {
                    module.hot.invalidate();
                }
                else {
                    self.$RefreshHelpers$.scheduleUpdate();
                }
            }
        }
        else {
            // Since we just executed the code for the module, it's possible that the
            // new exports made it ineligible for being a boundary.
            // We only care about the case when we were _previously_ a boundary,
            // because we already accepted this update (accidental side effect).
            var isNoLongerABoundary = prevExports !== null;
            if (isNoLongerABoundary) {
                module.hot.invalidate();
            }
        }
    }

/* WEBPACK VAR INJECTION */}.call(this, __webpack_require__(/*! ./../node_modules/next/dist/compiled/webpack/harmony-module.js */ "./node_modules/next/dist/compiled/webpack/harmony-module.js")(module)))

/***/ })

})
//# sourceMappingURL=data:application/json;charset=utf-8;base64,eyJ2ZXJzaW9uIjozLCJzb3VyY2VzIjpbIndlYnBhY2s6Ly9fTl9FLy4vY29tcG9uZW50cy9NZXNzYWdlLnRzeCIsIndlYnBhY2s6Ly9fTl9FLy4vY29tcG9uZW50cy9TdGFuZGluZ1Jvb20udHN4Il0sIm5hbWVzIjpbIk1lc3NhZ2UiLCJwcm9wcyIsImRlZmF1bHRfcm9vbV9zdGF0ZSIsIm5vX3NlYXRfcm9vbV9zdGF0ZSIsIm51bVdvcmtlcnMiLCJzZWF0cyIsImxlbmd0aCIsInN0eWxlcyIsIm1lc3NhZ2UiLCJSZWFjdCIsIkNvbXBvbmVudCIsIlN0YW5kaW5nUm9vbSIsIm51bVN0YW5kaW5nV29ya2VycyIsInN0YW5kaW5nUm9vbSIsImNvbW1hbmRTdHJpbmciXSwibWFwcGluZ3MiOiI7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7QUFBQTtBQUNBOztJQUdNQSxPOzs7Ozs7Ozs7Ozs7OzZCQUlLO0FBQ1AsVUFBSSxLQUFLQyxLQUFMLENBQVdDLGtCQUFYLElBQWlDLEtBQUtELEtBQUwsQ0FBV0Usa0JBQWhELEVBQW9FO0FBQ2xFLFlBQU1DLFVBQVUsR0FDZCxLQUFLSCxLQUFMLENBQVdDLGtCQUFYLENBQThCRyxLQUE5QixDQUFvQ0MsTUFBcEMsR0FDQSxLQUFLTCxLQUFMLENBQVdFLGtCQUFYLENBQThCRSxLQUE5QixDQUFvQ0MsTUFGdEM7QUFHQSw0QkFDRTtBQUFLLFlBQUUsRUFBRUMsMkRBQU0sQ0FBQ0MsT0FBaEI7QUFBQSxtQ0FBb0NKLFVBQXBDO0FBQUE7QUFBQTtBQUFBO0FBQUE7QUFBQSxnQkFERixDQUprRSxDQU9sRTtBQUNELE9BUkQsTUFRTztBQUNMLDRCQUFPO0FBQUssWUFBRSxFQUFFRywyREFBTSxDQUFDQztBQUFoQjtBQUFBO0FBQUE7QUFBQTtBQUFBLGdCQUFQO0FBQ0Q7QUFDRjs7OztFQWhCbUJDLDRDQUFLLENBQUNDLFM7O0FBbUJiVixzRUFBZjs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7O0FDdkJBO0FBQ0E7O0lBR01XLFk7Ozs7Ozs7Ozs7Ozs7NkJBSUs7QUFDUCxVQUFJLEtBQUtWLEtBQUwsQ0FBV0Usa0JBQWYsRUFBbUM7QUFDakMsWUFBTVMsa0JBQWtCLEdBQUcsS0FBS1gsS0FBTCxDQUFXRSxrQkFBWCxDQUE4QkUsS0FBOUIsQ0FBb0NDLE1BQS9EO0FBQ0EsNEJBQ0U7QUFBSyxZQUFFLEVBQUVDLGdFQUFNLENBQUNNLFlBQWhCO0FBQUEsa0NBQ0U7QUFBQTtBQUFBO0FBQUE7QUFBQTtBQUFBO0FBQUEsa0JBREYsZUFFRTtBQUFBO0FBQUE7QUFBQTtBQUFBO0FBQUE7QUFBQSxrQkFGRixlQUlFO0FBQUEsOENBQ0c7QUFBTSx1QkFBUyxFQUFFTixnRUFBTSxDQUFDTyxhQUF4QjtBQUFBO0FBQUE7QUFBQTtBQUFBO0FBQUE7QUFBQSxvQkFESDtBQUFBO0FBQUE7QUFBQTtBQUFBO0FBQUEsa0JBSkYsZUFPRTtBQUFBLHVCQUFLRixrQkFBTDtBQUFBO0FBQUE7QUFBQTtBQUFBO0FBQUEsa0JBUEY7QUFBQTtBQUFBO0FBQUE7QUFBQTtBQUFBLGdCQURGO0FBV0QsT0FiRCxNQWFPO0FBQ0wsNEJBQU87QUFBSyxZQUFFLEVBQUVMLGdFQUFNLENBQUNNO0FBQWhCO0FBQUE7QUFBQTtBQUFBO0FBQUEsZ0JBQVA7QUFDRDtBQUNGOzs7O0VBckJ3QkosNENBQUssQ0FBQ0MsUzs7QUF3QmxCQywyRUFBZiIsImZpbGUiOiJzdGF0aWMvd2VicGFjay9wYWdlcy9pbmRleC40N2UyNWExNDUwMjlmNDM1OTk3ZS5ob3QtdXBkYXRlLmpzIiwic291cmNlc0NvbnRlbnQiOlsiaW1wb3J0IFJlYWN0IGZyb20gXCJyZWFjdFwiO1xuaW1wb3J0IHN0eWxlcyBmcm9tIFwiLi9NZXNzYWdlLm1vZHVsZS5zYXNzXCI7XG5pbXBvcnQgeyBEZWZhdWx0Um9vbVN0YXRlLCBOb1NlYXRSb29tU3RhdGUgfSBmcm9tIFwiLi4vdHlwZXMvcm9vbS1zdGF0ZVwiO1xuXG5jbGFzcyBNZXNzYWdlIGV4dGVuZHMgUmVhY3QuQ29tcG9uZW50PFxuICB7IGRlZmF1bHRfcm9vbV9zdGF0ZTogRGVmYXVsdFJvb21TdGF0ZTsgbm9fc2VhdF9yb29tX3N0YXRlOiBOb1NlYXRSb29tU3RhdGUgfSxcbiAgYW55XG4+IHtcbiAgcmVuZGVyKCkge1xuICAgIGlmICh0aGlzLnByb3BzLmRlZmF1bHRfcm9vbV9zdGF0ZSAmJiB0aGlzLnByb3BzLm5vX3NlYXRfcm9vbV9zdGF0ZSkge1xuICAgICAgY29uc3QgbnVtV29ya2VycyA9XG4gICAgICAgIHRoaXMucHJvcHMuZGVmYXVsdF9yb29tX3N0YXRlLnNlYXRzLmxlbmd0aCArXG4gICAgICAgIHRoaXMucHJvcHMubm9fc2VhdF9yb29tX3N0YXRlLnNlYXRzLmxlbmd0aDtcbiAgICAgIHJldHVybiAoXG4gICAgICAgIDxkaXYgaWQ9e3N0eWxlcy5tZXNzYWdlfT5DdXJyZW50bHkge251bVdvcmtlcnN9IHBlb3BsZSB3b3JraW5nISDwn5SlPC9kaXY+XG4gICAgICApO1xuICAgICAgLy8gdG9kbyByZXR1cm4gPGRpdiBpZD17c3R5bGVzLm1lc3NhZ2V9PuePvuWcqOOAgXtudW1Xb3JrZXJzfeS6uuOBjOS9nOalreS4rfCflKU8L2Rpdj47XG4gICAgfSBlbHNlIHtcbiAgICAgIHJldHVybiA8ZGl2IGlkPXtzdHlsZXMubWVzc2FnZX0gLz47XG4gICAgfVxuICB9XG59XG5cbmV4cG9ydCBkZWZhdWx0IE1lc3NhZ2U7XG4iLCJpbXBvcnQgUmVhY3QgZnJvbSBcInJlYWN0XCI7XG5pbXBvcnQgc3R5bGVzIGZyb20gXCIuL1N0YW5kaW5nUm9vbS5tb2R1bGUuc2Fzc1wiO1xuaW1wb3J0IHsgTm9TZWF0Um9vbVN0YXRlIH0gZnJvbSBcIi4uL3R5cGVzL3Jvb20tc3RhdGVcIjtcblxuY2xhc3MgU3RhbmRpbmdSb29tIGV4dGVuZHMgUmVhY3QuQ29tcG9uZW50PFxuICB7IG5vX3NlYXRfcm9vbV9zdGF0ZTogTm9TZWF0Um9vbVN0YXRlIH0sXG4gIGFueVxuPiB7XG4gIHJlbmRlcigpIHtcbiAgICBpZiAodGhpcy5wcm9wcy5ub19zZWF0X3Jvb21fc3RhdGUpIHtcbiAgICAgIGNvbnN0IG51bVN0YW5kaW5nV29ya2VycyA9IHRoaXMucHJvcHMubm9fc2VhdF9yb29tX3N0YXRlLnNlYXRzLmxlbmd0aDtcbiAgICAgIHJldHVybiAoXG4gICAgICAgIDxkaXYgaWQ9e3N0eWxlcy5zdGFuZGluZ1Jvb219PlxuICAgICAgICAgIDxoMj7jgIA8L2gyPlxuICAgICAgICAgIDxoMj5TdGFuZGluZyBSb29tPC9oMj5cbiAgICAgICAgICB7LyogdG9kbyA8aDI+44K544K/44Oz44OH44Kj44Oz44Kw44Or44O844OgPC9oMj4qL31cbiAgICAgICAgICA8aDM+XG4gICAgICAgICAgICDvvIg8c3BhbiBjbGFzc05hbWU9e3N0eWxlcy5jb21tYW5kU3RyaW5nfT4hMDwvc3Bhbj4g44Gn5YWl5a6k77yJXG4gICAgICAgICAgPC9oMz5cbiAgICAgICAgICA8aDI+e251bVN0YW5kaW5nV29ya2Vyc33kuro8L2gyPlxuICAgICAgICA8L2Rpdj5cbiAgICAgICk7XG4gICAgfSBlbHNlIHtcbiAgICAgIHJldHVybiA8ZGl2IGlkPXtzdHlsZXMuc3RhbmRpbmdSb29tfSAvPjtcbiAgICB9XG4gIH1cbn1cblxuZXhwb3J0IGRlZmF1bHQgU3RhbmRpbmdSb29tO1xuIl0sInNvdXJjZVJvb3QiOiIifQ==