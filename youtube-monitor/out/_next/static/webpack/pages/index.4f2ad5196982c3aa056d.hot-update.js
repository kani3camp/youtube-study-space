webpackHotUpdate_N_E("pages/index",{

/***/ "./components/Clock.tsx":
/*!******************************!*\
  !*** ./components/Clock.tsx ***!
  \******************************/
/*! exports provided: default */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
__webpack_require__.r(__webpack_exports__);
/* WEBPACK VAR INJECTION */(function(module) {/* harmony import */ var react_jsx_dev_runtime__WEBPACK_IMPORTED_MODULE_0__ = __webpack_require__(/*! react/jsx-dev-runtime */ "./node_modules/react/jsx-dev-runtime.js");
/* harmony import */ var react_jsx_dev_runtime__WEBPACK_IMPORTED_MODULE_0___default = /*#__PURE__*/__webpack_require__.n(react_jsx_dev_runtime__WEBPACK_IMPORTED_MODULE_0__);
/* harmony import */ var C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_classCallCheck__WEBPACK_IMPORTED_MODULE_1__ = __webpack_require__(/*! ./node_modules/@babel/runtime/helpers/esm/classCallCheck */ "./node_modules/@babel/runtime/helpers/esm/classCallCheck.js");
/* harmony import */ var C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_createClass__WEBPACK_IMPORTED_MODULE_2__ = __webpack_require__(/*! ./node_modules/@babel/runtime/helpers/esm/createClass */ "./node_modules/@babel/runtime/helpers/esm/createClass.js");
/* harmony import */ var C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_assertThisInitialized__WEBPACK_IMPORTED_MODULE_3__ = __webpack_require__(/*! ./node_modules/@babel/runtime/helpers/esm/assertThisInitialized */ "./node_modules/@babel/runtime/helpers/esm/assertThisInitialized.js");
/* harmony import */ var C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_inherits__WEBPACK_IMPORTED_MODULE_4__ = __webpack_require__(/*! ./node_modules/@babel/runtime/helpers/esm/inherits */ "./node_modules/@babel/runtime/helpers/esm/inherits.js");
/* harmony import */ var C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_possibleConstructorReturn__WEBPACK_IMPORTED_MODULE_5__ = __webpack_require__(/*! ./node_modules/@babel/runtime/helpers/esm/possibleConstructorReturn */ "./node_modules/@babel/runtime/helpers/esm/possibleConstructorReturn.js");
/* harmony import */ var C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_getPrototypeOf__WEBPACK_IMPORTED_MODULE_6__ = __webpack_require__(/*! ./node_modules/@babel/runtime/helpers/esm/getPrototypeOf */ "./node_modules/@babel/runtime/helpers/esm/getPrototypeOf.js");
/* harmony import */ var C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_defineProperty__WEBPACK_IMPORTED_MODULE_7__ = __webpack_require__(/*! ./node_modules/@babel/runtime/helpers/esm/defineProperty */ "./node_modules/@babel/runtime/helpers/esm/defineProperty.js");
/* harmony import */ var react__WEBPACK_IMPORTED_MODULE_8__ = __webpack_require__(/*! react */ "./node_modules/react/index.js");
/* harmony import */ var react__WEBPACK_IMPORTED_MODULE_8___default = /*#__PURE__*/__webpack_require__.n(react__WEBPACK_IMPORTED_MODULE_8__);
/* harmony import */ var _Clock_module_sass__WEBPACK_IMPORTED_MODULE_9__ = __webpack_require__(/*! ./Clock.module.sass */ "./components/Clock.module.sass");
/* harmony import */ var _Clock_module_sass__WEBPACK_IMPORTED_MODULE_9___default = /*#__PURE__*/__webpack_require__.n(_Clock_module_sass__WEBPACK_IMPORTED_MODULE_9__);








var _jsxFileName = "C:\\Users\\momom\\Documents\\GitHub\\youtube-study-space\\youtube-monitor\\components\\Clock.tsx";

function _createSuper(Derived) { var hasNativeReflectConstruct = _isNativeReflectConstruct(); return function _createSuperInternal() { var Super = Object(C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_getPrototypeOf__WEBPACK_IMPORTED_MODULE_6__["default"])(Derived), result; if (hasNativeReflectConstruct) { var NewTarget = Object(C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_getPrototypeOf__WEBPACK_IMPORTED_MODULE_6__["default"])(this).constructor; result = Reflect.construct(Super, arguments, NewTarget); } else { result = Super.apply(this, arguments); } return Object(C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_possibleConstructorReturn__WEBPACK_IMPORTED_MODULE_5__["default"])(this, result); }; }

function _isNativeReflectConstruct() { if (typeof Reflect === "undefined" || !Reflect.construct) return false; if (Reflect.construct.sham) return false; if (typeof Proxy === "function") return true; try { Date.prototype.toString.call(Reflect.construct(Date, [], function () {})); return true; } catch (e) { return false; } }




var Clock = /*#__PURE__*/function (_React$Component) {
  Object(C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_inherits__WEBPACK_IMPORTED_MODULE_4__["default"])(Clock, _React$Component);

  var _super = _createSuper(Clock);

  function Clock(props) {
    var _this;

    Object(C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_classCallCheck__WEBPACK_IMPORTED_MODULE_1__["default"])(this, Clock);

    _this = _super.call(this, props);

    Object(C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_defineProperty__WEBPACK_IMPORTED_MODULE_7__["default"])(Object(C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_assertThisInitialized__WEBPACK_IMPORTED_MODULE_3__["default"])(_this), "intervalId", void 0);

    _this.state = {
      now: new Date()
    };
    return _this;
  }

  Object(C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_createClass__WEBPACK_IMPORTED_MODULE_2__["default"])(Clock, [{
    key: "componentDidMount",
    value: function componentDidMount() {
      var _this2 = this;

      this.intervalId = setInterval(function () {
        _this2.setState({
          now: new Date()
        });
      }, 1000);
    }
  }, {
    key: "componentWillUnmount",
    value: function componentWillUnmount() {
      if (this.intervalId) {
        clearInterval(this.intervalId);
      }
    }
  }, {
    key: "render",
    value: function render() {
      return /*#__PURE__*/Object(react_jsx_dev_runtime__WEBPACK_IMPORTED_MODULE_0__["jsxDEV"])("div", {
        id: _Clock_module_sass__WEBPACK_IMPORTED_MODULE_9___default.a.clock,
        children: [/*#__PURE__*/Object(react_jsx_dev_runtime__WEBPACK_IMPORTED_MODULE_0__["jsxDEV"])("div", {
          className: _Clock_module_sass__WEBPACK_IMPORTED_MODULE_9___default.a.dateString,
          children: [this.state.now.getMonth(), " / ", this.state.now.getDate(), " /", " ", this.state.now.getFullYear()]
        }, void 0, true, {
          fileName: _jsxFileName,
          lineNumber: 31,
          columnNumber: 9
        }, this), /*#__PURE__*/Object(react_jsx_dev_runtime__WEBPACK_IMPORTED_MODULE_0__["jsxDEV"])("div", {
          className: _Clock_module_sass__WEBPACK_IMPORTED_MODULE_9___default.a.timeString,
          children: [this.state.now.getHours(), "\uFF1A", this.state.now.getMinutes() < 10 ? "0" + this.state.now.getMinutes().toString() : this.state.now.getMinutes()]
        }, void 0, true, {
          fileName: _jsxFileName,
          lineNumber: 38,
          columnNumber: 9
        }, this)]
      }, void 0, true, {
        fileName: _jsxFileName,
        lineNumber: 30,
        columnNumber: 7
      }, this);
    }
  }]);

  return Clock;
}(react__WEBPACK_IMPORTED_MODULE_8___default.a.Component);

/* harmony default export */ __webpack_exports__["default"] = (Clock);

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
//# sourceMappingURL=data:application/json;charset=utf-8;base64,eyJ2ZXJzaW9uIjozLCJzb3VyY2VzIjpbIndlYnBhY2s6Ly9fTl9FLy4vY29tcG9uZW50cy9DbG9jay50c3giXSwibmFtZXMiOlsiQ2xvY2siLCJwcm9wcyIsInN0YXRlIiwibm93IiwiRGF0ZSIsImludGVydmFsSWQiLCJzZXRJbnRlcnZhbCIsInNldFN0YXRlIiwiY2xlYXJJbnRlcnZhbCIsInN0eWxlcyIsImNsb2NrIiwiZGF0ZVN0cmluZyIsImdldE1vbnRoIiwiZ2V0RGF0ZSIsImdldEZ1bGxZZWFyIiwidGltZVN0cmluZyIsImdldEhvdXJzIiwiZ2V0TWludXRlcyIsInRvU3RyaW5nIiwiUmVhY3QiLCJDb21wb25lbnQiXSwibWFwcGluZ3MiOiI7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7O0FBQUE7QUFDQTs7SUFFTUEsSzs7Ozs7QUFHSixpQkFBWUMsS0FBWixFQUF1QjtBQUFBOztBQUFBOztBQUNyQiw4QkFBTUEsS0FBTjs7QUFEcUI7O0FBRXJCLFVBQUtDLEtBQUwsR0FBYTtBQUNYQyxTQUFHLEVBQUUsSUFBSUMsSUFBSjtBQURNLEtBQWI7QUFGcUI7QUFLdEI7Ozs7d0NBRW1CO0FBQUE7O0FBQ2xCLFdBQUtDLFVBQUwsR0FBa0JDLFdBQVcsQ0FBQyxZQUFNO0FBQ2xDLGNBQUksQ0FBQ0MsUUFBTCxDQUFjO0FBQ1pKLGFBQUcsRUFBRSxJQUFJQyxJQUFKO0FBRE8sU0FBZDtBQUdELE9BSjRCLEVBSTFCLElBSjBCLENBQTdCO0FBS0Q7OzsyQ0FFc0I7QUFDckIsVUFBSSxLQUFLQyxVQUFULEVBQXFCO0FBQ25CRyxxQkFBYSxDQUFDLEtBQUtILFVBQU4sQ0FBYjtBQUNEO0FBQ0Y7Ozs2QkFFUTtBQUNQLDBCQUNFO0FBQUssVUFBRSxFQUFFSSx5REFBTSxDQUFDQyxLQUFoQjtBQUFBLGdDQUNFO0FBQUssbUJBQVMsRUFBRUQseURBQU0sQ0FBQ0UsVUFBdkI7QUFBQSxxQkFDRyxLQUFLVCxLQUFMLENBQVdDLEdBQVgsQ0FBZVMsUUFBZixFQURILFNBQ2lDLEtBQUtWLEtBQUwsQ0FBV0MsR0FBWCxDQUFlVSxPQUFmLEVBRGpDLFFBQzZELEdBRDdELEVBRUcsS0FBS1gsS0FBTCxDQUFXQyxHQUFYLENBQWVXLFdBQWYsRUFGSDtBQUFBO0FBQUE7QUFBQTtBQUFBO0FBQUEsZ0JBREYsZUFRRTtBQUFLLG1CQUFTLEVBQUVMLHlEQUFNLENBQUNNLFVBQXZCO0FBQUEscUJBQ0csS0FBS2IsS0FBTCxDQUFXQyxHQUFYLENBQWVhLFFBQWYsRUFESCxZQUVHLEtBQUtkLEtBQUwsQ0FBV0MsR0FBWCxDQUFlYyxVQUFmLEtBQThCLEVBQTlCLEdBQ0csTUFBTSxLQUFLZixLQUFMLENBQVdDLEdBQVgsQ0FBZWMsVUFBZixHQUE0QkMsUUFBNUIsRUFEVCxHQUVHLEtBQUtoQixLQUFMLENBQVdDLEdBQVgsQ0FBZWMsVUFBZixFQUpOO0FBQUE7QUFBQTtBQUFBO0FBQUE7QUFBQSxnQkFSRjtBQUFBO0FBQUE7QUFBQTtBQUFBO0FBQUEsY0FERjtBQWlCRDs7OztFQTFDaUJFLDRDQUFLLENBQUNDLFM7O0FBNkNYcEIsb0VBQWYiLCJmaWxlIjoic3RhdGljL3dlYnBhY2svcGFnZXMvaW5kZXguNGYyYWQ1MTk2OTgyYzNhYTA1NmQuaG90LXVwZGF0ZS5qcyIsInNvdXJjZXNDb250ZW50IjpbImltcG9ydCBSZWFjdCBmcm9tIFwicmVhY3RcIjtcbmltcG9ydCBzdHlsZXMgZnJvbSBcIi4vQ2xvY2subW9kdWxlLnNhc3NcIjtcblxuY2xhc3MgQ2xvY2sgZXh0ZW5kcyBSZWFjdC5Db21wb25lbnQ8e30sIGFueT4ge1xuICBwcml2YXRlIGludGVydmFsSWQ6IE5vZGVKUy5UaW1lb3V0IHwgdW5kZWZpbmVkO1xuXG4gIGNvbnN0cnVjdG9yKHByb3BzOiB7fSkge1xuICAgIHN1cGVyKHByb3BzKTtcbiAgICB0aGlzLnN0YXRlID0ge1xuICAgICAgbm93OiBuZXcgRGF0ZSgpLFxuICAgIH07XG4gIH1cblxuICBjb21wb25lbnREaWRNb3VudCgpIHtcbiAgICB0aGlzLmludGVydmFsSWQgPSBzZXRJbnRlcnZhbCgoKSA9PiB7XG4gICAgICB0aGlzLnNldFN0YXRlKHtcbiAgICAgICAgbm93OiBuZXcgRGF0ZSgpLFxuICAgICAgfSk7XG4gICAgfSwgMTAwMCk7XG4gIH1cblxuICBjb21wb25lbnRXaWxsVW5tb3VudCgpIHtcbiAgICBpZiAodGhpcy5pbnRlcnZhbElkKSB7XG4gICAgICBjbGVhckludGVydmFsKHRoaXMuaW50ZXJ2YWxJZCk7XG4gICAgfVxuICB9XG5cbiAgcmVuZGVyKCkge1xuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IGlkPXtzdHlsZXMuY2xvY2t9PlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT17c3R5bGVzLmRhdGVTdHJpbmd9PlxuICAgICAgICAgIHt0aGlzLnN0YXRlLm5vdy5nZXRNb250aCgpfSAvIHt0aGlzLnN0YXRlLm5vdy5nZXREYXRlKCl9IC97XCIgXCJ9XG4gICAgICAgICAge3RoaXMuc3RhdGUubm93LmdldEZ1bGxZZWFyKCl9XG4gICAgICAgICAgey8qe3RoaXMuc3RhdGUubm93LmdldEZ1bGxZZWFyKCl9IOW5tCB7JyAnfSovfVxuICAgICAgICAgIHsvKnt0aGlzLnN0YXRlLm5vdy5nZXRNb250aCgpfSDmnIggeycgJ30qL31cbiAgICAgICAgICB7Lyp7dGhpcy5zdGF0ZS5ub3cuZ2V0RGF0ZSgpfSDml6UqL31cbiAgICAgICAgPC9kaXY+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPXtzdHlsZXMudGltZVN0cmluZ30+XG4gICAgICAgICAge3RoaXMuc3RhdGUubm93LmdldEhvdXJzKCl977yaXG4gICAgICAgICAge3RoaXMuc3RhdGUubm93LmdldE1pbnV0ZXMoKSA8IDEwXG4gICAgICAgICAgICA/IFwiMFwiICsgdGhpcy5zdGF0ZS5ub3cuZ2V0TWludXRlcygpLnRvU3RyaW5nKClcbiAgICAgICAgICAgIDogdGhpcy5zdGF0ZS5ub3cuZ2V0TWludXRlcygpfVxuICAgICAgICA8L2Rpdj5cbiAgICAgIDwvZGl2PlxuICAgICk7XG4gIH1cbn1cblxuZXhwb3J0IGRlZmF1bHQgQ2xvY2s7XG4iXSwic291cmNlUm9vdCI6IiJ9