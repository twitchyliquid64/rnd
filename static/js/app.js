var app = angular.module('kcdb', ['ui.materialize', 'angularMoment']);

app.controller('BodyController', ["$scope", "$rootScope", function ($scope, $rootScope) {
    $scope.page = "home";
    $scope.changePage = function(pageName){
        $scope.page = pageName;
        $rootScope.$broadcast('page-change', {page: pageName});
    };
}]);

app.controller('HomeController', ["$scope", "$http", "$rootScope", "$interval", function ($scope, $http, $rootScope, $interval) {
    $scope.loading = true;

    $scope.loadStatus = function(query){
      $scope.loading = true;
      $scope.error = null;
      $http({
        method: 'GET',
        url: '/status',
        data: {query: $scope.searchQ},
      }).then(function successCallback(response) {
        $scope.status = response.data;
        $scope.loading = false;
      }, function errorCallback(response) {
        $scope.loading = false;
        $scope.error = response;
      });
    }

    // error info helpers.
    $scope.ec = function(){
      if (!$scope.error)return null;
      if ($scope.error.success === false)
        return 'N/A';
      return $scope.error.status;
    }
    $scope.exp = function(){
      if (!$scope.error)return null;
      if ($scope.error.status === -1)
        return "Network Error or server offline";
      if ($scope.error.success === false)
        return 'The server encountered a problem handling the request';
      return $scope.error.statusText;
    }

    $scope.loadStatus();
}]);
