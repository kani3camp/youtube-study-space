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
        var numWorkers = this.props.default_room_state.seats.length + this.props.no_seat_room_state.seats.length; // return (
        //   <div id={styles.message}>Currently {numWorkers} people working! 🔥</div>
        // );

        return /*#__PURE__*/Object(react_jsx_dev_runtime__WEBPACK_IMPORTED_MODULE_0__["jsxDEV"])("div", {
          id: _Message_module_sass__WEBPACK_IMPORTED_MODULE_7___default.a.message,
          children: ["\u73FE\u5728\u3001", numWorkers, "\u4EBA\u304C\u4F5C\u696D\u4E2D\uD83D\uDD25"]
        }, void 0, true, {
          fileName: _jsxFileName,
          lineNumber: 17,
          columnNumber: 14
        }, this);
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

/***/ })

})
//# sourceMappingURL=data:application/json;charset=utf-8;base64,eyJ2ZXJzaW9uIjozLCJzb3VyY2VzIjpbIndlYnBhY2s6Ly9fTl9FLy4vY29tcG9uZW50cy9NZXNzYWdlLnRzeCJdLCJuYW1lcyI6WyJNZXNzYWdlIiwicHJvcHMiLCJkZWZhdWx0X3Jvb21fc3RhdGUiLCJub19zZWF0X3Jvb21fc3RhdGUiLCJudW1Xb3JrZXJzIiwic2VhdHMiLCJsZW5ndGgiLCJzdHlsZXMiLCJtZXNzYWdlIiwiUmVhY3QiLCJDb21wb25lbnQiXSwibWFwcGluZ3MiOiI7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7QUFBQTtBQUNBOztJQUdNQSxPOzs7Ozs7Ozs7Ozs7OzZCQUlLO0FBQ1AsVUFBSSxLQUFLQyxLQUFMLENBQVdDLGtCQUFYLElBQWlDLEtBQUtELEtBQUwsQ0FBV0Usa0JBQWhELEVBQW9FO0FBQ2xFLFlBQU1DLFVBQVUsR0FDZCxLQUFLSCxLQUFMLENBQVdDLGtCQUFYLENBQThCRyxLQUE5QixDQUFvQ0MsTUFBcEMsR0FDQSxLQUFLTCxLQUFMLENBQVdFLGtCQUFYLENBQThCRSxLQUE5QixDQUFvQ0MsTUFGdEMsQ0FEa0UsQ0FJbEU7QUFDQTtBQUNBOztBQUNBLDRCQUFPO0FBQUssWUFBRSxFQUFFQywyREFBTSxDQUFDQyxPQUFoQjtBQUFBLDJDQUE2QkosVUFBN0I7QUFBQTtBQUFBO0FBQUE7QUFBQTtBQUFBLGdCQUFQO0FBQ0QsT0FSRCxNQVFPO0FBQ0wsNEJBQU87QUFBSyxZQUFFLEVBQUVHLDJEQUFNLENBQUNDO0FBQWhCO0FBQUE7QUFBQTtBQUFBO0FBQUEsZ0JBQVA7QUFDRDtBQUNGOzs7O0VBaEJtQkMsNENBQUssQ0FBQ0MsUzs7QUFtQmJWLHNFQUFmIiwiZmlsZSI6InN0YXRpYy93ZWJwYWNrL3BhZ2VzL2luZGV4LmM0ZDdiNjcwOGM5YTI1NDMxZTQ5LmhvdC11cGRhdGUuanMiLCJzb3VyY2VzQ29udGVudCI6WyJpbXBvcnQgUmVhY3QgZnJvbSBcInJlYWN0XCI7XG5pbXBvcnQgc3R5bGVzIGZyb20gXCIuL01lc3NhZ2UubW9kdWxlLnNhc3NcIjtcbmltcG9ydCB7IERlZmF1bHRSb29tU3RhdGUsIE5vU2VhdFJvb21TdGF0ZSB9IGZyb20gXCIuLi90eXBlcy9yb29tLXN0YXRlXCI7XG5cbmNsYXNzIE1lc3NhZ2UgZXh0ZW5kcyBSZWFjdC5Db21wb25lbnQ8XG4gIHsgZGVmYXVsdF9yb29tX3N0YXRlOiBEZWZhdWx0Um9vbVN0YXRlOyBub19zZWF0X3Jvb21fc3RhdGU6IE5vU2VhdFJvb21TdGF0ZSB9LFxuICBhbnlcbj4ge1xuICByZW5kZXIoKSB7XG4gICAgaWYgKHRoaXMucHJvcHMuZGVmYXVsdF9yb29tX3N0YXRlICYmIHRoaXMucHJvcHMubm9fc2VhdF9yb29tX3N0YXRlKSB7XG4gICAgICBjb25zdCBudW1Xb3JrZXJzID1cbiAgICAgICAgdGhpcy5wcm9wcy5kZWZhdWx0X3Jvb21fc3RhdGUuc2VhdHMubGVuZ3RoICtcbiAgICAgICAgdGhpcy5wcm9wcy5ub19zZWF0X3Jvb21fc3RhdGUuc2VhdHMubGVuZ3RoO1xuICAgICAgLy8gcmV0dXJuIChcbiAgICAgIC8vICAgPGRpdiBpZD17c3R5bGVzLm1lc3NhZ2V9PkN1cnJlbnRseSB7bnVtV29ya2Vyc30gcGVvcGxlIHdvcmtpbmchIPCflKU8L2Rpdj5cbiAgICAgIC8vICk7XG4gICAgICByZXR1cm4gPGRpdiBpZD17c3R5bGVzLm1lc3NhZ2V9PuePvuWcqOOAgXtudW1Xb3JrZXJzfeS6uuOBjOS9nOalreS4rfCflKU8L2Rpdj47XG4gICAgfSBlbHNlIHtcbiAgICAgIHJldHVybiA8ZGl2IGlkPXtzdHlsZXMubWVzc2FnZX0gLz47XG4gICAgfVxuICB9XG59XG5cbmV4cG9ydCBkZWZhdWx0IE1lc3NhZ2U7XG4iXSwic291cmNlUm9vdCI6IiJ9